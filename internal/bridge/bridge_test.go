package bridge

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// mockWorkerPath returns the absolute path to mock_worker.py relative to this file.
func mockWorkerPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	// internal/bridge/ -> repo root -> python/mock_worker.py
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(root, "python", "mock_worker.py")
}

func pythonBin(t *testing.T) string {
	t.Helper()
	for _, bin := range []string{"python3", "python"} {
		if path, err := exec.LookPath(bin); err == nil {
			return path
		}
	}
	t.Skip("no python3/python binary found — skipping bridge tests")
	return ""
}

func newTestBridge(t *testing.T) *Bridge {
	t.Helper()
	py := pythonBin(t)
	script := mockWorkerPath(t)
	b := New(py, script)
	if err := b.Start(); err != nil {
		t.Fatalf("bridge.Start: %v", err)
	}
	t.Cleanup(func() { b.Stop() })
	return b
}

// mockRequest builds an InferenceRequest where Params is raw JSON so we can
// inject mock control fields (_mock_error, _mock_progress) that the mock worker reads.
func mockRequest(id string, extra map[string]any) InferenceRequest {
	// Build params map with base fields + extras
	params := map[string]any{
		"caption":         "A lo-fi beat",
		"lyrics":          "[Instrumental]",
		"instrumental":    false,
		"vocal_language":  "en",
		"inference_steps": 8,
		"guidance_scale":  7.0,
		"seed":            -1,
		"thinking":        true,
	}
	for k, v := range extra {
		params[k] = v
	}

	// Round-trip through JSON to get an InferenceParams value.
	// The mock worker reads the raw JSON so extra fields are preserved.
	// We store them in a RawParams field — but InferenceParams doesn't have one.
	// Instead we use a rawBridgeRequest (unexported) that the test sends directly.
	_ = params
	return InferenceRequest{
		ID:       id,
		TaskType: "text2music",
		SaveDir:  "/tmp",
		Params: InferenceParams{
			Caption:        "A lo-fi beat",
			Lyrics:         "[Instrumental]",
			InferenceSteps: 8,
			GuidanceScale:  7.0,
			Seed:           -1,
			Thinking:       true,
		},
		Config: InferenceConfig{BatchSize: 1, AudioFormat: "flac", Seed: -1},
	}
}

// sendRaw sends a raw JSON object to the bridge worker and reads the response.
// Used to inject mock control fields that InferenceRequest doesn't expose.
func sendRaw(t *testing.T, b *Bridge, raw map[string]any, progressCh chan<- ProgressEvent) (*InferenceResponse, error) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.enc.Encode(raw); err != nil {
		return nil, err
	}

	for {
		if !b.scanner.Scan() {
			if err := b.scanner.Err(); err != nil {
				return nil, err
			}
			return nil, nil
		}
		line := b.scanner.Bytes()
		var wl workerLine
		if err := json.Unmarshal(line, &wl); err != nil {
			continue
		}
		if wl.Type == "progress" {
			if progressCh != nil {
				select {
				case progressCh <- ProgressEvent{Value: wl.Value, Stage: wl.Stage}:
				default:
				}
			}
			continue
		}
		return &InferenceResponse{
			Success:     wl.Success,
			Error:       wl.Error,
			LMAvailable: wl.LMAvailable,
		}, nil
	}
}

func TestBridge_HappyPath(t *testing.T) {
	b := newTestBridge(t)

	req := InferenceRequest{
		ID:       "test-1",
		TaskType: "text2music",
		SaveDir:  t.TempDir(),
		Params:   InferenceParams{Caption: "A lo-fi beat", Lyrics: "[Instrumental]", InferenceSteps: 8, GuidanceScale: 7.0, Seed: -1, Thinking: true},
		Config:   InferenceConfig{BatchSize: 1, AudioFormat: "flac", Seed: -1},
	}

	resp, err := b.Generate(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got error: %s", resp.Error)
	}
	if !resp.LMAvailable {
		t.Error("expected lm_available=true from mock worker")
	}
}

func TestBridge_ErrorResponse(t *testing.T) {
	b := newTestBridge(t)

	raw := map[string]any{
		"id":        "test-err",
		"task_type": "text2music",
		"save_dir":  t.TempDir(),
		"params": map[string]any{
			"caption":      "test",
			"_mock_error":  "inference exploded",
		},
		"config": map[string]any{"batch_size": 1, "audio_format": "flac", "seed": -1},
	}

	resp, err := sendRaw(t, b, raw, nil)
	if err != nil {
		t.Fatalf("sendRaw: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "inference exploded" {
		t.Errorf("expected error %q, got %q", "inference exploded", resp.Error)
	}
}

func TestBridge_ProgressEvents(t *testing.T) {
	b := newTestBridge(t)

	progressCh := make(chan ProgressEvent, 10)
	raw := map[string]any{
		"id":        "test-prog",
		"task_type": "text2music",
		"save_dir":  t.TempDir(),
		"params": map[string]any{
			"caption":         "test",
			"_mock_progress":  3,
		},
		"config": map[string]any{"batch_size": 1, "audio_format": "flac", "seed": -1},
	}

	resp, err := sendRaw(t, b, raw, progressCh)
	if err != nil {
		t.Fatalf("sendRaw: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.Error)
	}

	close(progressCh)
	var events []ProgressEvent
	for e := range progressCh {
		events = append(events, e)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 progress events, got %d", len(events))
	}
	for i, e := range events {
		if e.Value <= 0 || e.Value > 1 {
			t.Errorf("event %d: value %f out of range", i, e.Value)
		}
		if e.Stage == "" {
			t.Errorf("event %d: empty stage", i)
		}
	}
}

func TestBridge_MultipleRequests(t *testing.T) {
	b := newTestBridge(t)

	for i := 0; i < 3; i++ {
		req := InferenceRequest{
			ID:       "multi-" + string(rune('0'+i)),
			TaskType: "text2music",
			SaveDir:  t.TempDir(),
			Params:   InferenceParams{Caption: "test", InferenceSteps: 8, GuidanceScale: 7.0, Seed: -1},
			Config:   InferenceConfig{BatchSize: 1, AudioFormat: "flac", Seed: -1},
		}
		resp, err := b.Generate(context.Background(), req, nil)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if !resp.Success {
			t.Errorf("request %d: expected success", i)
		}
	}
}

func TestBridge_CoverRequest(t *testing.T) {
	b := newTestBridge(t)

	req := InferenceRequest{
		ID:       "cover-1",
		TaskType: "cover",
		SaveDir:  t.TempDir(),
		Params: InferenceParams{
			Caption:            "Jazz style",
			ReferenceAudio:     "/tmp/ref.flac",
			AudioCoverStrength: 0.5,
			InferenceSteps:     8,
			GuidanceScale:      7.0,
			Seed:               -1,
		},
		Config: InferenceConfig{BatchSize: 1, AudioFormat: "flac", Seed: -1},
	}

	resp, err := b.Generate(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.Error)
	}
}

func TestBridge_RepaintRequest(t *testing.T) {
	b := newTestBridge(t)

	req := InferenceRequest{
		ID:       "repaint-1",
		TaskType: "repaint",
		SaveDir:  t.TempDir(),
		Params: InferenceParams{
			Caption:         "Brighter chorus",
			SrcAudio:        "/tmp/src.flac",
			RepaintingStart: 10.0,
			RepaintingEnd:   20.0,
			InferenceSteps:  8,
			GuidanceScale:   7.0,
			Seed:            -1,
		},
		Config: InferenceConfig{BatchSize: 1, AudioFormat: "flac", Seed: -1},
	}

	resp, err := b.Generate(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.Error)
	}
}
