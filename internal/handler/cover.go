package handler

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/schema"
)

// Cover handles POST /cover (style transfer with reference audio).
func (h *Handlers) Cover(w http.ResponseWriter, r *http.Request) {
	// Hard cap on request body size (100 MB) to prevent disk exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	saveDir, cleanup, err := h.makeTempDir("music_gen_cover_")
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

	// Save uploaded reference audio
	refFile, refHeader, err := r.FormFile("reference_audio")
	if err != nil {
		writeError(w, http.StatusBadRequest, "reference_audio file is required")
		return
	}
	defer refFile.Close()

	refExt := filepath.Ext(refHeader.Filename)
	refPath := filepath.Join(saveDir, "reference_audio"+refExt)
	if err := saveUpload(refFile, refPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save reference audio")
		return
	}

	// Parse form fields with defaults
	caption := r.FormValue("caption")
	if caption == "" {
		writeError(w, http.StatusBadRequest, "caption must not be empty")
		return
	}

	req := schema.GenerateRequest{
		Caption:       caption,
		Lyrics:        formValueOr(r, "lyrics", "[Instrumental]"),
		Instrumental:  formBool(r, "instrumental", false),
		VocalLanguage: formValueOr(r, "vocal_language", "en"),
		Duration:      formFloat64Ptr(r, "duration"),
		BPM:           formIntPtr(r, "bpm"),
		KeyScale:      formStringPtr(r, "keyscale"),
		TimeSignature: formStringPtr(r, "timesignature"),
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

	audioCoverStrength := formFloat64(r, "audio_cover_strength", 0.5)
	if audioCoverStrength < 0.0 || audioCoverStrength > 1.0 {
		writeError(w, http.StatusBadRequest, "audio_cover_strength must be between 0.0 and 1.0")
		return
	}

	if err := schema.Validate(&req, h.Cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("cover: caption=%q strength=%.2f", truncate(caption, 80), audioCoverStrength)

	start := time.Now()
	ireq := bridge.InferenceRequest{
		ID:       newRequestID(),
		TaskType: "cover",
		SaveDir:  saveDir,
		Params: bridge.InferenceParams{
			Caption:            req.Caption,
			Lyrics:             req.Lyrics,
			Instrumental:       req.Instrumental,
			VocalLanguage:      req.VocalLanguage,
			Duration:           req.Duration,
			BPM:                req.BPM,
			KeyScale:           req.KeyScale,
			TimeSignature:      req.TimeSignature,
			InferenceSteps:     req.InferenceSteps,
			GuidanceScale:      req.GuidanceScale,
			Seed:               *req.Seed,
			Thinking:           req.Thinking,
			ReferenceAudio:     refPath,
			AudioCoverStrength: audioCoverStrength,
		},
		Config: bridge.InferenceConfig{
			BatchSize:   req.BatchSize,
			AudioFormat: req.AudioFormat,
			Seed:        *req.Seed,
		},
	}

	resp, err := h.Bridge.Generate(r.Context(), ireq, nil)
	if err != nil {
		log.Printf("cover: bridge error: %v", err)
		writeError(w, http.StatusInternalServerError, "Cover generation failed")
		return
	}
	if !resp.Success {
		log.Printf("cover: inference error: %s", resp.Error)
		writeError(w, http.StatusInternalServerError, "Cover generation failed")
		return
	}

	audios := collectAudioFiles(saveDir, req.AudioFormat)
	if audios == nil {
		audios = []string{}
	}
	elapsed := time.Since(start).Seconds()
	log.Printf("cover: produced %d audios in %.2fs", len(audios), elapsed)

	writeJSON(w, http.StatusOK, schema.AudioResponse{
		Audios: audios,
		Metadata: schema.InferenceMetadata{
			RequestType:   "cover",
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

// ---------------------------------------------------------------------------
// multipart form helpers
// ---------------------------------------------------------------------------

func formValueOr(r *http.Request, key, def string) string {
	if v := r.FormValue(key); v != "" {
		return v
	}
	return def
}

func formBool(r *http.Request, key string, def bool) bool {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func formInt(r *http.Request, key string, def int) int {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func formInt64(r *http.Request, key string, def int64) int64 {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func formFloat64(r *http.Request, key string, def float64) float64 {
	v := r.FormValue(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func formFloat64Ptr(r *http.Request, key string) *float64 {
	v := r.FormValue(key)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func formIntPtr(r *http.Request, key string) *int {
	v := r.FormValue(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func formStringPtr(r *http.Request, key string) *string {
	v := r.FormValue(key)
	if v == "" {
		return nil
	}
	return &v
}
