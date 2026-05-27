package handler

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/schema"
)

// Repaint handles POST /repaint (selective region editing).
func (h *Handlers) Repaint(w http.ResponseWriter, r *http.Request) {
	// Hard cap on request body size (100 MB) to prevent disk exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	saveDir, cleanup, err := h.makeTempDir("music_gen_repaint_")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp dir")
		return
	}
	defer cleanup()

	// Validate batch_size before touching the file upload
	if bs := formInt(r, "batch_size", 1); bs > h.Cfg.MaxBatchSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batch_size must be <= %d", h.Cfg.MaxBatchSize))
		return
	}

	// Save uploaded source audio
	srcFile, srcHeader, err := r.FormFile("src_audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "src_audio file is required")
		return
	}
	defer srcFile.Close()

	srcExt := filepath.Ext(srcHeader.Filename)
	srcPath := filepath.Join(saveDir, "src_audio"+srcExt)
	if err := saveUpload(srcFile, srcPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save source audio")
		return
	}

	// Parse required repaint range
	repaintingStart := formFloat64(r, "repainting_start", 0.0)
	repaintingEnd := formFloat64(r, "repainting_end", 0.0)

	if repaintingEnd <= repaintingStart {
		writeError(w, http.StatusBadRequest, "repainting_end must be > repainting_start")
		return
	}

	caption := r.FormValue("caption")
	if caption == "" {
		writeError(w, http.StatusBadRequest, "caption must not be empty")
		return
	}

	req := schema.GenerateRequest{
		Caption:        caption,
		Lyrics:         formValueOr(r, "lyrics", "[Instrumental]"),
		Instrumental:   formBool(r, "instrumental", false),
		VocalLanguage:  formValueOr(r, "vocal_language", "en"),
		Duration:       formFloat64Ptr(r, "duration"),
		BPM:            formIntPtr(r, "bpm"),
		KeyScale:       formStringPtr(r, "keyscale"),
		TimeSignature:  formStringPtr(r, "timesignature"),
		InferenceSteps: formInt(r, "inference_steps", h.Cfg.DefaultInferenceSteps),
		GuidanceScale:  formFloat64(r, "guidance_scale", h.Cfg.DefaultGuidanceScale),
		Seed:           func() *int64 { s := formInt64(r, "seed", -1); return &s }(),
		BatchSize:      formInt(r, "batch_size", 1),
		AudioFormat:    formValueOr(r, "audio_format", h.Cfg.DefaultAudioFormat),
		Thinking:       formBool(r, "thinking", true),
	}
	if req.Duration == nil {
		d := h.Cfg.DefaultDuration
		req.Duration = &d
	}

	if err := schema.Validate(&req, h.Cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("repaint: caption=%q range=%.1f-%.1f", truncate(caption, 80), repaintingStart, repaintingEnd)

	start := time.Now()
	ireq := bridge.InferenceRequest{
		ID:       newRequestID(),
		TaskType: "repaint",
		SaveDir:  saveDir,
		Params: bridge.InferenceParams{
			Caption:         req.Caption,
			Lyrics:          req.Lyrics,
			Instrumental:    req.Instrumental,
			VocalLanguage:   req.VocalLanguage,
			Duration:        req.Duration,
			BPM:             req.BPM,
			KeyScale:        req.KeyScale,
			TimeSignature:   req.TimeSignature,
			InferenceSteps:  req.InferenceSteps,
			GuidanceScale:   req.GuidanceScale,
			Seed:            *req.Seed,
			Thinking:        req.Thinking,
			SrcAudio:        srcPath,
			RepaintingStart: repaintingStart,
			RepaintingEnd:   repaintingEnd,
		},
		Config: bridge.InferenceConfig{
			BatchSize:   req.BatchSize,
			AudioFormat: req.AudioFormat,
			Seed:        *req.Seed,
		},
	}

	resp, err := h.Bridge.Generate(r.Context(), ireq, nil)
	if err != nil {
		log.Printf("repaint: bridge error: %v", err)
		writeError(w, http.StatusInternalServerError, "Repaint generation failed")
		return
	}
	if !resp.Success {
		log.Printf("repaint: inference error: %s", resp.Error)
		writeError(w, http.StatusInternalServerError, "Repaint generation failed")
		return
	}

	audios := collectAudioFiles(saveDir, req.AudioFormat)
	if audios == nil {
		audios = []string{}
	}
	elapsed := time.Since(start).Seconds()
	log.Printf("repaint: produced %d audios in %.2fs", len(audios), elapsed)

	writeJSON(w, http.StatusOK, schema.AudioResponse{
		Audios: audios,
		Metadata: schema.InferenceMetadata{
			RequestType:   "repaint",
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
