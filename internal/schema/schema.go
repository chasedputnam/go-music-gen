package schema

import (
	"fmt"
	"strings"

	"github.com/chasedputnam/go-music-gen/internal/config"
)

// GenerateRequest is the JSON body for POST /generate and POST /generate/stream.
type GenerateRequest struct {
	Caption        string   `json:"caption"`
	Lyrics         string   `json:"lyrics"`
	Instrumental   bool     `json:"instrumental"`
	VocalLanguage  string   `json:"vocal_language"`
	Duration       *float64 `json:"duration"`
	BPM            *int     `json:"bpm"`
	KeyScale       *string  `json:"keyscale"`
	TimeSignature  *string  `json:"timesignature"`
	InferenceSteps int      `json:"inference_steps"`
	GuidanceScale  float64  `json:"guidance_scale"`
	Seed           *int64   `json:"seed"` // nil means "not provided" — defaults to -1 (random)
	BatchSize      int      `json:"batch_size"`
	AudioFormat    string   `json:"audio_format"`
	Thinking       bool     `json:"thinking"`
}

// InferenceMetadata is included in every AudioResponse.
type InferenceMetadata struct {
	RequestType   string   `json:"request_type"`
	DitConfig     string   `json:"dit_config"`
	Device        string   `json:"device"`
	Dtype         string   `json:"dtype"`
	Caption       string   `json:"caption"`
	Duration      *float64 `json:"duration"`
	Steps         int      `json:"steps"`
	GuidanceScale float64  `json:"guidance_scale"`
	Seed          int64    `json:"seed"` // resolved seed (-1 = random)
	Elapsed       float64  `json:"elapsed"`
	NumAudios     int      `json:"num_audios"`
	AudioFormat   string   `json:"audio_format"`
	LMEnabled     bool     `json:"lm_enabled"`
}

// AudioResponse is returned by /generate, /cover, and /repaint.
type AudioResponse struct {
	Audios   []string          `json:"audios"`
	Metadata InferenceMetadata `json:"metadata"`
}

// ErrorResponse is returned on 4xx/5xx errors.
type ErrorResponse struct {
	Detail string `json:"detail"`
}

var allowedAudioFormats = map[string]bool{
	"mp3": true, "wav": true, "flac": true,
	"wav32": true, "opus": true, "aac": true,
}

var allowedTimeSignatures = map[string]bool{
	"2/4": true, "3/4": true, "4/4": true, "6/8": true,
}

// ApplyDefaults fills zero-value fields with config-driven defaults.
// Call this before Validate.
func ApplyDefaults(req *GenerateRequest, cfg *config.Settings) {
	if req.Lyrics == "" {
		req.Lyrics = "[Instrumental]"
	}
	if req.VocalLanguage == "" {
		req.VocalLanguage = "en"
	}
	if req.Duration == nil {
		d := cfg.DefaultDuration
		req.Duration = &d
	}
	if req.InferenceSteps == 0 {
		req.InferenceSteps = cfg.DefaultInferenceSteps
	}
	if req.GuidanceScale == 0 {
		req.GuidanceScale = cfg.DefaultGuidanceScale
	}
	if req.AudioFormat == "" {
		req.AudioFormat = cfg.DefaultAudioFormat
	}
	if req.BatchSize == 0 {
		req.BatchSize = 1
	}
	if req.Seed == nil {
		defaultSeed := int64(-1)
		req.Seed = &defaultSeed
	}
	// Note: Thinking cannot be defaulted to true here because JSON bool zero-value
	// is false and is indistinguishable from an omitted field. Clients that want
	// thinking=true must set it explicitly in the JSON body.
}

// Validate checks all field constraints from Requirement 2.
// Returns a non-nil error with a human-readable message on the first failing constraint.
func Validate(req *GenerateRequest, cfg *config.Settings) error {
	if strings.TrimSpace(req.Caption) == "" {
		return fmt.Errorf("caption must not be empty")
	}
	if len(req.Caption) > 512 {
		return fmt.Errorf("caption must not exceed 512 characters")
	}
	if len(req.Lyrics) > 4096 {
		return fmt.Errorf("lyrics must not exceed 4096 characters")
	}
	if req.Duration != nil {
		if *req.Duration < 10 || *req.Duration > 600 {
			return fmt.Errorf("duration must be between 10 and 600 seconds")
		}
	}
	if req.BPM != nil {
		if *req.BPM < 30 || *req.BPM > 300 {
			return fmt.Errorf("bpm must be between 30 and 300")
		}
	}
	if req.InferenceSteps < 1 || req.InferenceSteps > 100 {
		return fmt.Errorf("inference_steps must be between 1 and 100")
	}
	if req.GuidanceScale < 0.0 || req.GuidanceScale > 20.0 {
		return fmt.Errorf("guidance_scale must be between 0.0 and 20.0")
	}
	if req.Seed != nil && (*req.Seed < -1 || *req.Seed > 4294967295) {
		return fmt.Errorf("seed must be between -1 and 4294967295")
	}
	if !allowedAudioFormats[req.AudioFormat] {
		return fmt.Errorf("audio_format must be one of: mp3, wav, flac, wav32, opus, aac")
	}
	if req.TimeSignature != nil && !allowedTimeSignatures[*req.TimeSignature] {
		return fmt.Errorf("timesignature must be one of: 2/4, 3/4, 4/4, 6/8")
	}
	if req.BatchSize > cfg.MaxBatchSize {
		return fmt.Errorf("batch_size must be <= %d", cfg.MaxBatchSize)
	}
	if req.BatchSize < 1 {
		return fmt.Errorf("batch_size must be >= 1")
	}
	return nil
}
