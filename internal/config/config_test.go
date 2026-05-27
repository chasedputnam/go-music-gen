package config

import (
	"os"
	"testing"
)

func setenv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestDefaults(t *testing.T) {
	// Clear all relevant env vars so defaults apply
	keys := []string{
		"DIT_CONFIG", "LM_MODEL_PATH", "ENABLE_LM", "HOST", "PORT",
		"DEFAULT_DURATION", "MAX_DURATION", "NUM_INFERENCE_STEPS",
		"GUIDANCE_SCALE", "AUDIO_FORMAT", "MAX_BATCH_SIZE",
		"DEVICE", "MODEL_PRECISION", "LM_BACKEND", "PRELOAD_MODELS", "MUSIC_ENV",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}

	s := Load()

	if s.DitConfig != "acestep-v15-turbo" {
		t.Errorf("DitConfig: got %q, want %q", s.DitConfig, "acestep-v15-turbo")
	}
	if s.LmModelPath != "acestep-5Hz-lm-1.7B" {
		t.Errorf("LmModelPath: got %q, want %q", s.LmModelPath, "acestep-5Hz-lm-1.7B")
	}
	if !s.EnableLM {
		t.Error("EnableLM: want true by default")
	}
	if s.Host != "0.0.0.0" {
		t.Errorf("Host: got %q, want %q", s.Host, "0.0.0.0")
	}
	if s.Port != 4009 {
		t.Errorf("Port: got %d, want 4009", s.Port)
	}
	if s.DefaultDuration != 30 {
		t.Errorf("DefaultDuration: got %f, want 30", s.DefaultDuration)
	}
	if s.MaxDuration != 600 {
		t.Errorf("MaxDuration: got %f, want 600", s.MaxDuration)
	}
	if s.DefaultInferenceSteps != 8 {
		t.Errorf("DefaultInferenceSteps: got %d, want 8", s.DefaultInferenceSteps)
	}
	if s.DefaultGuidanceScale != 7.0 {
		t.Errorf("DefaultGuidanceScale: got %f, want 7.0", s.DefaultGuidanceScale)
	}
	if s.DefaultAudioFormat != "flac" {
		t.Errorf("DefaultAudioFormat: got %q, want %q", s.DefaultAudioFormat, "flac")
	}
	if s.MaxBatchSize != 2 {
		t.Errorf("MaxBatchSize: got %d, want 2", s.MaxBatchSize)
	}
	if s.Device != "" {
		t.Errorf("Device: got %q, want empty", s.Device)
	}
	if s.ModelPrecision != "" {
		t.Errorf("ModelPrecision: got %q, want empty", s.ModelPrecision)
	}
	if s.LmBackend != "" {
		t.Errorf("LmBackend: got %q, want empty", s.LmBackend)
	}
	if s.DevMode {
		t.Error("DevMode: want false by default")
	}
	if !s.PreloadModels {
		t.Error("PreloadModels: want true in production (default)")
	}
}

func TestEnvOverrides(t *testing.T) {
	setenv(t, "DIT_CONFIG", "my-dit")
	setenv(t, "LM_MODEL_PATH", "my-lm")
	setenv(t, "HOST", "127.0.0.1")
	setenv(t, "PORT", "8080")
	setenv(t, "DEFAULT_DURATION", "60")
	setenv(t, "MAX_DURATION", "300")
	setenv(t, "NUM_INFERENCE_STEPS", "20")
	setenv(t, "GUIDANCE_SCALE", "5.5")
	setenv(t, "AUDIO_FORMAT", "mp3")
	setenv(t, "MAX_BATCH_SIZE", "4")
	setenv(t, "DEVICE", "cuda")
	setenv(t, "MODEL_PRECISION", "float16")
	setenv(t, "LM_BACKEND", "mlx")

	s := Load()

	if s.DitConfig != "my-dit" {
		t.Errorf("DitConfig: got %q", s.DitConfig)
	}
	if s.LmModelPath != "my-lm" {
		t.Errorf("LmModelPath: got %q", s.LmModelPath)
	}
	if s.Host != "127.0.0.1" {
		t.Errorf("Host: got %q", s.Host)
	}
	if s.Port != 8080 {
		t.Errorf("Port: got %d", s.Port)
	}
	if s.DefaultDuration != 60 {
		t.Errorf("DefaultDuration: got %f", s.DefaultDuration)
	}
	if s.MaxDuration != 300 {
		t.Errorf("MaxDuration: got %f", s.MaxDuration)
	}
	if s.DefaultInferenceSteps != 20 {
		t.Errorf("DefaultInferenceSteps: got %d", s.DefaultInferenceSteps)
	}
	if s.DefaultGuidanceScale != 5.5 {
		t.Errorf("DefaultGuidanceScale: got %f", s.DefaultGuidanceScale)
	}
	if s.DefaultAudioFormat != "mp3" {
		t.Errorf("DefaultAudioFormat: got %q", s.DefaultAudioFormat)
	}
	if s.MaxBatchSize != 4 {
		t.Errorf("MaxBatchSize: got %d", s.MaxBatchSize)
	}
	if s.Device != "cuda" {
		t.Errorf("Device: got %q", s.Device)
	}
	if s.ModelPrecision != "float16" {
		t.Errorf("ModelPrecision: got %q", s.ModelPrecision)
	}
	if s.LmBackend != "mlx" {
		t.Errorf("LmBackend: got %q", s.LmBackend)
	}
}

func TestEnableLMFalsy(t *testing.T) {
	cases := []string{"0", "false", "False"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			setenv(t, "ENABLE_LM", v)
			s := Load()
			if s.EnableLM {
				t.Errorf("EnableLM: want false for ENABLE_LM=%q", v)
			}
		})
	}
}

func TestDevMode(t *testing.T) {
	setenv(t, "MUSIC_ENV", "development")
	os.Unsetenv("PRELOAD_MODELS")
	s := Load()
	if !s.DevMode {
		t.Error("DevMode: want true when MUSIC_ENV=development")
	}
	if s.PreloadModels {
		t.Error("PreloadModels: want false in dev mode by default")
	}
}

func TestPreloadModelsExplicit(t *testing.T) {
	setenv(t, "MUSIC_ENV", "development")
	setenv(t, "PRELOAD_MODELS", "1")
	s := Load()
	if !s.PreloadModels {
		t.Error("PreloadModels: want true when PRELOAD_MODELS=1 even in dev mode")
	}

	setenv(t, "MUSIC_ENV", "production")
	setenv(t, "PRELOAD_MODELS", "0")
	s = Load()
	if s.PreloadModels {
		t.Error("PreloadModels: want false when PRELOAD_MODELS=0 even in prod mode")
	}
}
