package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/schema"
)

// Generate handles POST /generate (text-to-music).
func (h *Handlers) Generate(w http.ResponseWriter, r *http.Request) {
	var req schema.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	schema.ApplyDefaults(&req, h.Cfg)

	if err := schema.Validate(&req, h.Cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("generate: caption=%q duration=%v steps=%d guidance=%.1f",
		truncate(req.Caption, 80), req.Duration, req.InferenceSteps, req.GuidanceScale)

	saveDir, cleanup, err := h.makeTempDir("music_gen_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer cleanup()

	start := time.Now()
	ireq := bridge.InferenceRequest{
		ID:       newRequestID(),
		TaskType: "text2music",
		SaveDir:  saveDir,
		Params: bridge.InferenceParams{
			Caption:        req.Caption,
			Lyrics:         req.Lyrics,
			Instrumental:   req.Instrumental,
			VocalLanguage:  req.VocalLanguage,
			Duration:       req.Duration,
			BPM:            req.BPM,
			KeyScale:       req.KeyScale,
			TimeSignature:  req.TimeSignature,
			InferenceSteps: req.InferenceSteps,
			GuidanceScale:  req.GuidanceScale,
			Seed:           *req.Seed,
			Thinking:       req.Thinking,
		},
		Config: bridge.InferenceConfig{
			BatchSize:   req.BatchSize,
			AudioFormat: req.AudioFormat,
			Seed:        *req.Seed,
		},
	}

	resp, err := h.Bridge.Generate(r.Context(), ireq, nil)
	if err != nil {
		log.Printf("generate: bridge error: %v", err)
		writeError(w, http.StatusInternalServerError, "Music generation failed")
		return
	}
	if !resp.Success {
		log.Printf("generate: inference error: %s", resp.Error)
		writeError(w, http.StatusInternalServerError, "Music generation failed")
		return
	}

	audios := collectAudioFiles(saveDir, req.AudioFormat)
	if audios == nil {
		audios = []string{}
	}
	elapsed := time.Since(start).Seconds()
	log.Printf("generate: produced %d audios in %.2fs", len(audios), elapsed)

	writeJSON(w, http.StatusOK, schema.AudioResponse{
		Audios: audios,
		Metadata: schema.InferenceMetadata{
			RequestType:   "text2music",
			DitConfig:     h.Cfg.DitConfig,
			Device:        h.Cfg.Device,
			Dtype:         h.Cfg.ModelPrecision,
			Caption:       req.Caption,
			Duration:      req.Duration,
			Steps:         req.InferenceSteps,
			GuidanceScale: req.GuidanceScale,
			Seed:          *req.Seed,
			Elapsed:       elapsed,
			NumAudios:     len(audios),
			AudioFormat:   req.AudioFormat,
			LMEnabled:     h.Bridge.LMAvailable(),
		},
	})
}
