package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

//go:embed inference_worker.py
var inferenceWorkerScript string

const maxRestarts = 1

// Bridge manages the Python inference worker subprocess and serializes all
// inference calls through a single mutex (GPU memory cannot be shared).
type Bridge struct {
	mu          sync.Mutex
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	enc         *json.Encoder
	scanner     *bufio.Scanner
	scriptPath  string // path to the extracted temp script file
	pythonBin   string
	restarts    int
	lmAvailable bool
	device      string // resolved by Python worker (e.g. "cuda", "mps", "cpu")
	dtype       string // resolved by Python worker (e.g. "bfloat16", "float32")
	scriptFile  string // temp file written on Start(), deleted on Stop()
}

// New creates a Bridge. pythonBin is the Python interpreter path (e.g. "python3").
// If scriptPath is non-empty it is used directly (useful for tests); otherwise
// the embedded inference_worker.py is written to a temp file on Start().
func New(pythonBin, scriptPath string) *Bridge {
	return &Bridge{
		pythonBin:  pythonBin,
		scriptPath: scriptPath,
	}
}

// LMAvailable returns whether the LLM was successfully loaded by the worker.
func (b *Bridge) LMAvailable() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lmAvailable
}

// Device returns the compute device resolved by the Python worker (e.g. "cuda", "mps", "cpu").
// Returns empty string until the first successful inference or preload.
func (b *Bridge) Device() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.device
}

// Dtype returns the torch dtype resolved by the Python worker (e.g. "bfloat16", "float32").
// Returns empty string until the first successful inference or preload.
func (b *Bridge) Dtype() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dtype
}

// Start launches the Python worker subprocess.
func (b *Bridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.start()
}

// start launches the subprocess. Must be called with b.mu held.
func (b *Bridge) start() error {
	scriptPath := b.scriptPath

	// If no explicit script path, extract the embedded script to a temp file.
	if scriptPath == "" {
		f, err := os.CreateTemp("", "inference_worker_*.py")
		if err != nil {
			return fmt.Errorf("bridge: create temp script: %w", err)
		}
		if _, err := f.WriteString(inferenceWorkerScript); err != nil {
			f.Close()
			os.Remove(f.Name())
			return fmt.Errorf("bridge: write temp script: %w", err)
		}
		f.Close()
		scriptPath = f.Name()
		b.scriptFile = f.Name()
	}

	cmd := exec.Command(b.pythonBin, scriptPath)
	cmd.Stderr = os.Stderr
	// Pass the working directory as project root so the Python worker can find
	// ACE-Step checkpoints regardless of where the script file lives (e.g. a temp file).
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cmd.Env = append(os.Environ(), "MUSIC_PROJECT_ROOT="+cwd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("bridge: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("bridge: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("bridge: start subprocess: %w", err)
	}

	b.cmd = cmd
	b.stdin = stdin
	b.enc = json.NewEncoder(stdin)
	b.scanner = bufio.NewScanner(stdout)
	// Increase scanner buffer for large progress/result lines
	b.scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	log.Printf("bridge: worker started (pid=%d)", cmd.Process.Pid)
	return nil
}

// Stop terminates the Python worker subprocess gracefully.
func (b *Bridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stop()
}

// stop shuts down the subprocess. Must be called with b.mu held.
func (b *Bridge) stop() {
	if b.stdin != nil {
		b.stdin.Close()
		b.stdin = nil
	}
	if b.cmd != nil && b.cmd.Process != nil {
		b.cmd.Process.Kill()
		b.cmd.Wait()
		b.cmd = nil
	}
	// Clean up the extracted temp script if we created it
	if b.scriptFile != "" {
		os.Remove(b.scriptFile)
		b.scriptFile = ""
	}
}

// Generate sends an inference request to the Python worker and returns the result.
// Progress events are sent to progressCh (may be nil). The call blocks until the
// worker emits a result line. All calls are serialized by the internal mutex.
func (b *Bridge) Generate(ctx context.Context, req InferenceRequest, progressCh chan<- ProgressEvent) (*InferenceResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	resp, err := b.generate(ctx, req, progressCh)
	if err != nil && b.restarts < maxRestarts {
		// Attempt one restart on subprocess failure
		log.Printf("bridge: worker error (%v), attempting restart %d/%d", err, b.restarts+1, maxRestarts)
		b.stop()
		if startErr := b.start(); startErr != nil {
			return nil, fmt.Errorf("bridge: restart failed: %w", startErr)
		}
		b.restarts++
		resp, err = b.generate(ctx, req, progressCh)
	}
	if err == nil {
		// Reset restart counter after a successful call so future crashes
		// are retried regardless of how many restarts have occurred historically.
		b.restarts = 0
	}
	return resp, err
}

// generate performs the actual JSON-RPC exchange. Must be called with b.mu held.
func (b *Bridge) generate(ctx context.Context, req InferenceRequest, progressCh chan<- ProgressEvent) (*InferenceResponse, error) {
	if b.cmd == nil {
		return nil, fmt.Errorf("bridge: worker not running")
	}

	// Send request
	if err := b.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("bridge: encode request: %w", err)
	}

	// Read lines until we get a result (non-progress) line
	for {
		// Check context cancellation between reads
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !b.scanner.Scan() {
			if err := b.scanner.Err(); err != nil {
				return nil, fmt.Errorf("bridge: read stdout: %w", err)
			}
			return nil, fmt.Errorf("bridge: worker closed stdout unexpectedly")
		}

		line := b.scanner.Bytes()
		var wl workerLine
		if err := json.Unmarshal(line, &wl); err != nil {
			log.Printf("bridge: unparseable line from worker: %s", line)
			continue
		}

		if wl.Type == "progress" {
			if progressCh != nil {
				select {
				case progressCh <- ProgressEvent{Value: wl.Value, Stage: wl.Stage}:
				default: // drop if channel full — non-fatal
				}
			}
			continue
		}

		// Result line
		if wl.LMAvailable {
			b.lmAvailable = true
		}
		if wl.Device != "" {
			b.device = wl.Device
		}
		if wl.Dtype != "" {
			b.dtype = wl.Dtype
		}
		return &InferenceResponse{
			Success:     wl.Success,
			Error:       wl.Error,
			LMAvailable: wl.LMAvailable,
			Device:      wl.Device,
			Dtype:       wl.Dtype,
		}, nil
	}
}
