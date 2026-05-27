package config

import (
	"os"
	"strconv"
	"strings"
)

// Settings holds all runtime configuration resolved from environment variables.
type Settings struct {
	// ACE-Step model configuration
	DitConfig   string
	LmModelPath string
	EnableLM    bool

	// Server
	Host string
	Port int

	// Audio defaults
	DefaultDuration       float64
	MaxDuration           float64
	DefaultInferenceSteps int
	DefaultGuidanceScale  float64
	DefaultAudioFormat    string
	MaxBatchSize          int

	// Device / precision
	Device         string
	ModelPrecision string
	LmBackend      string

	// Runtime mode
	PreloadModels bool
	DevMode       bool
}

// Load reads all configuration from environment variables and returns a Settings
// instance with defaults applied where env vars are absent.
func Load() *Settings {
	devMode := os.Getenv("MUSIC_ENV") == "development"

	preload := !devMode // default: true in prod, false in dev
	if v := os.Getenv("PRELOAD_MODELS"); v != "" {
		preload = !isFalsy(v)
	}

	return &Settings{
		DitConfig:             envOr("DIT_CONFIG", "acestep-v15-turbo"),
		LmModelPath:           envOr("LM_MODEL_PATH", "acestep-5Hz-lm-1.7B"),
		EnableLM:              !isFalsy(envOr("ENABLE_LM", "1")),
		Host:                  envOr("HOST", "0.0.0.0"),
		Port:                  envInt("PORT", 4009),
		DefaultDuration:       envFloat("DEFAULT_DURATION", 30),
		MaxDuration:           envFloat("MAX_DURATION", 600),
		DefaultInferenceSteps: envInt("NUM_INFERENCE_STEPS", 8),
		DefaultGuidanceScale:  envFloat("GUIDANCE_SCALE", 7.0),
		DefaultAudioFormat:    envOr("AUDIO_FORMAT", "flac"),
		MaxBatchSize:          envInt("MAX_BATCH_SIZE", 2),
		Device:                os.Getenv("DEVICE"),
		ModelPrecision:        os.Getenv("MODEL_PRECISION"),
		LmBackend:             os.Getenv("LM_BACKEND"),
		PreloadModels:         preload,
		DevMode:               devMode,
	}
}

// isFalsy returns true if the value represents a disabled/false state.
func isFalsy(v string) bool {
	lower := strings.ToLower(strings.TrimSpace(v))
	return lower == "0" || lower == "false" || lower == ""
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
