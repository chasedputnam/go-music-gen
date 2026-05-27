package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/config"
	"github.com/chasedputnam/go-music-gen/internal/schema"
)

// testHandlers builds a Handlers wired to the mock Python worker.
func testHandlers(t *testing.T) *Handlers {
	t.Helper()
	py := findPython(t)
	script := mockScript(t)
	b := bridge.New(py, script)
	if err := b.Start(); err != nil {
		t.Fatalf("bridge.Start: %v", err)
	}
	t.Cleanup(func() { b.Stop() })

	cfg := config.Load()
	cfg.MaxBatchSize = 2
	cfg.DefaultAudioFormat = "flac"
	cfg.DefaultDuration = 30
	cfg.DefaultInferenceSteps = 8
	cfg.DefaultGuidanceScale = 7.0

	return &Handlers{Cfg: cfg, Bridge: b}
}

func findPython(t *testing.T) string {
	t.Helper()
	for _, bin := range []string{"python3", "python"} {
		if p, err := exec.LookPath(bin); err == nil {
			return p
		}
	}
	t.Skip("no python3/python binary found")
	return ""
}

func mockScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return filepath.Join(root, "python", "mock_worker.py")
}

// postJSON sends a POST request with a JSON body and returns the recorder.
func postJSON(t *testing.T, h *Handlers, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	return rr
}

func TestGenerate_HappyPath(t *testing.T) {
	h := testHandlers(t)

	// Create a fake audio file in the save dir so collectAudioFiles returns something.
	// We can't control the save dir from outside, so we test that the response shape is correct
	// and audios is an empty slice (mock worker doesn't write files).
	rr := postJSON(t, h, "/generate", map[string]any{
		"caption":         "A lo-fi beat",
		"duration":        15.0,
		"inference_steps": 8,
		"guidance_scale":  7.0,
		"seed":            42,
		"instrumental":    true,
		"audio_format":    "flac",
		"batch_size":      1,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp schema.AudioResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Audios == nil {
		t.Error("audios should not be nil (should be empty slice, not null)")
	}
	// mock worker doesn't write files so audios is empty — that's expected
	_ = resp.Audios
	if resp.Metadata.RequestType != "text2music" {
		t.Errorf("request_type: got %q", resp.Metadata.RequestType)
	}
	if resp.Metadata.Elapsed <= 0 {
		t.Error("elapsed should be > 0")
	}
	if resp.Metadata.AudioFormat != "flac" {
		t.Errorf("audio_format: got %q", resp.Metadata.AudioFormat)
	}
}

func TestGenerate_ValidationError_EmptyCaption(t *testing.T) {
	h := testHandlers(t)
	rr := postJSON(t, h, "/generate", map[string]any{
		"caption": "",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var errResp schema.ErrorResponse
	json.NewDecoder(rr.Body).Decode(&errResp)
	if errResp.Detail == "" {
		t.Error("expected non-empty detail in error response")
	}
}

func TestGenerate_ValidationError_BadAudioFormat(t *testing.T) {
	h := testHandlers(t)
	rr := postJSON(t, h, "/generate", map[string]any{
		"caption":      "test",
		"audio_format": "ogg",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGenerate_ValidationError_BatchSizeTooLarge(t *testing.T) {
	h := testHandlers(t)
	rr := postJSON(t, h, "/generate", map[string]any{
		"caption":    "test",
		"batch_size": 99,
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGenerate_AudioCollection(t *testing.T) {
	// Write fake audio files to a temp dir and verify collectAudioFiles picks them up.
	dir := t.TempDir()
	files := []string{"output_0.flac", "output_1.flac", "other.txt"}
	for _, f := range files {
		os.WriteFile(filepath.Join(dir, f), []byte("fakeaudio"), 0644)
	}
	audios := collectAudioFiles(dir, "flac")
	if len(audios) != 2 {
		t.Errorf("expected 2 audio files, got %d", len(audios))
	}
}

func TestGenerate_InvalidJSON(t *testing.T) {
	h := testHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/generate", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
