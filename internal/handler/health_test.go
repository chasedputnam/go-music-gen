package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/config"
)

func newTestHandlers() *Handlers {
	cfg := &config.Settings{
		Device:         "cpu",
		ModelPrecision: "float32",
		DitConfig:      "acestep-v15-turbo",
		LmModelPath:    "acestep-5Hz-lm-1.7B",
		EnableLM:       true,
		LmBackend:      "pt",
	}
	// Bridge with no subprocess — LMAvailable() returns false (zero value)
	b := bridge.New("python3", "/nonexistent")
	return &Handlers{Cfg: cfg, Bridge: b}
}

func TestHealth(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	fields := []string{"status", "device", "dtype", "dit_config", "lm_model", "lm_enabled", "lm_backend", "lm_available"}
	for _, f := range fields {
		if _, ok := body[f]; !ok {
			t.Errorf("missing field %q in health response", f)
		}
	}
	if body["status"] != "ok" {
		t.Errorf("status: got %v, want ok", body["status"])
	}
	if body["dit_config"] != "acestep-v15-turbo" {
		t.Errorf("dit_config: got %v", body["dit_config"])
	}
}
