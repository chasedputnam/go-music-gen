package schema

import (
	"strings"
	"testing"

	"github.com/chasedputnam/go-music-gen/internal/config"
)

func defaultCfg() *config.Settings {
	return &config.Settings{
		DefaultDuration:       30,
		MaxDuration:           600,
		DefaultInferenceSteps: 8,
		DefaultGuidanceScale:  7.0,
		DefaultAudioFormat:    "flac",
		MaxBatchSize:          2,
	}
}

func validReq() *GenerateRequest {
	d := 30.0
	return &GenerateRequest{
		Caption:        "A lo-fi beat",
		Lyrics:         "[Instrumental]",
		VocalLanguage:  "en",
		Duration:       &d,
		InferenceSteps: 8,
		GuidanceScale:  7.0,
		Seed:           func() *int64 { s := int64(-1); return &s }(),
		BatchSize:      1,
		AudioFormat:    "flac",
	}
}

func TestValidate_HappyPath(t *testing.T) {
	if err := Validate(validReq(), defaultCfg()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_CaptionEmpty(t *testing.T) {
	req := validReq()
	req.Caption = ""
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for empty caption")
	}
}

func TestValidate_CaptionWhitespace(t *testing.T) {
	req := validReq()
	req.Caption = "   "
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for whitespace-only caption")
	}
}

func TestValidate_CaptionTooLong(t *testing.T) {
	req := validReq()
	req.Caption = strings.Repeat("a", 513)
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for caption > 512 chars")
	}
	req.Caption = strings.Repeat("a", 512)
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for caption == 512 chars, got: %v", err)
	}
}

func TestValidate_LyricsTooLong(t *testing.T) {
	req := validReq()
	req.Lyrics = strings.Repeat("x", 4097)
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for lyrics > 4096 chars")
	}
	req.Lyrics = strings.Repeat("x", 4096)
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for lyrics == 4096 chars, got: %v", err)
	}
}

func TestValidate_DurationBounds(t *testing.T) {
	req := validReq()
	low := 9.9
	req.Duration = &low
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for duration < 10")
	}
	min := 10.0
	req.Duration = &min
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for duration == 10, got: %v", err)
	}
	high := 600.1
	req.Duration = &high
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for duration > 600")
	}
	max := 600.0
	req.Duration = &max
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for duration == 600, got: %v", err)
	}
}

func TestValidate_BPMBounds(t *testing.T) {
	req := validReq()
	low := 29
	req.BPM = &low
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for bpm < 30")
	}
	min := 30
	req.BPM = &min
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for bpm == 30, got: %v", err)
	}
	high := 301
	req.BPM = &high
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for bpm > 300")
	}
	max := 300
	req.BPM = &max
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for bpm == 300, got: %v", err)
	}
}

func TestValidate_InferenceStepsBounds(t *testing.T) {
	req := validReq()
	req.InferenceSteps = 0
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for inference_steps < 1")
	}
	req.InferenceSteps = 1
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for inference_steps == 1, got: %v", err)
	}
	req.InferenceSteps = 101
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for inference_steps > 100")
	}
	req.InferenceSteps = 100
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for inference_steps == 100, got: %v", err)
	}
}

func TestValidate_GuidanceScaleBounds(t *testing.T) {
	req := validReq()
	req.GuidanceScale = -0.1
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for guidance_scale < 0")
	}
	req.GuidanceScale = 0.0
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for guidance_scale == 0, got: %v", err)
	}
	req.GuidanceScale = 20.1
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for guidance_scale > 20")
	}
	req.GuidanceScale = 20.0
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for guidance_scale == 20, got: %v", err)
	}
}

func TestValidate_SeedBounds(t *testing.T) {
	req := validReq()
	s := int64(-2)
	req.Seed = &s
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for seed < -1")
	}
	s = -1
	req.Seed = &s
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for seed == -1, got: %v", err)
	}
	s = 4294967295
	req.Seed = &s
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for seed == max uint32, got: %v", err)
	}
	s = 4294967296
	req.Seed = &s
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for seed > max uint32")
	}
}

func TestValidate_AudioFormat(t *testing.T) {
	req := validReq()
	for _, fmt := range []string{"mp3", "wav", "flac", "wav32", "opus", "aac"} {
		req.AudioFormat = fmt
		if err := Validate(req, defaultCfg()); err != nil {
			t.Errorf("expected no error for audio_format=%q, got: %v", fmt, err)
		}
	}
	req.AudioFormat = "ogg"
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for invalid audio_format")
	}
}

func TestValidate_TimeSignature(t *testing.T) {
	req := validReq()
	for _, ts := range []string{"2/4", "3/4", "4/4", "6/8"} {
		req.TimeSignature = &ts
		if err := Validate(req, defaultCfg()); err != nil {
			t.Errorf("expected no error for timesignature=%q, got: %v", ts, err)
		}
	}
	bad := "5/4"
	req.TimeSignature = &bad
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for invalid timesignature")
	}
	req.TimeSignature = nil
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for nil timesignature, got: %v", err)
	}
}

func TestValidate_BatchSize(t *testing.T) {
	req := validReq()
	req.BatchSize = 2
	if err := Validate(req, defaultCfg()); err != nil {
		t.Fatalf("expected no error for batch_size == max, got: %v", err)
	}
	req.BatchSize = 3
	if err := Validate(req, defaultCfg()); err == nil {
		t.Fatal("expected error for batch_size > MAX_BATCH_SIZE")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := defaultCfg()
	req := &GenerateRequest{Caption: "test"}
	ApplyDefaults(req, cfg)

	if req.Lyrics != "[Instrumental]" {
		t.Errorf("Lyrics default: got %q", req.Lyrics)
	}
	if req.VocalLanguage != "en" {
		t.Errorf("VocalLanguage default: got %q", req.VocalLanguage)
	}
	if req.Duration == nil || *req.Duration != 30 {
		t.Errorf("Duration default: got %v", req.Duration)
	}
	if req.InferenceSteps != 8 {
		t.Errorf("InferenceSteps default: got %d", req.InferenceSteps)
	}
	if req.GuidanceScale != 7.0 {
		t.Errorf("GuidanceScale default: got %f", req.GuidanceScale)
	}
	if req.AudioFormat != "flac" {
		t.Errorf("AudioFormat default: got %q", req.AudioFormat)
	}
	if req.BatchSize != 1 {
		t.Errorf("BatchSize default: got %d", req.BatchSize)
	}
	if req.Seed == nil || *req.Seed != -1 {
		t.Errorf("Seed default: got %v", req.Seed)
	}
}
