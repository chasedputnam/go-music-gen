package handler

import (
	"net/http"
)

// Health handles GET /health.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	// Use bridge-resolved device/dtype when available (populated after first
	// preload or inference). Fall back to config values (env var overrides).
	device := h.Bridge.Device()
	if device == "" {
		device = h.Cfg.Device
	}
	dtype := h.Bridge.Dtype()
	if dtype == "" {
		dtype = h.Cfg.ModelPrecision
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"device":       device,
		"dtype":        dtype,
		"dit_config":   h.Cfg.DitConfig,
		"lm_model":     h.Cfg.LmModelPath,
		"lm_enabled":   h.Cfg.EnableLM,
		"lm_backend":   h.Cfg.LmBackend,
		"lm_available": h.Bridge.LMAvailable(),
	})
}
