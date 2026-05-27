package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// TestE2E_Generate posts a generate request to a live server and verifies the response.
// Skipped unless MUSIC_ENDPOINT is set.
func TestE2E_Generate(t *testing.T) {
	endpoint := os.Getenv("MUSIC_ENDPOINT")
	if endpoint == "" {
		t.Skip("MUSIC_ENDPOINT not set — skipping e2e test")
	}

	payload := map[string]any{
		"caption":         "A gentle lo-fi hip hop beat with warm piano chords and vinyl crackle",
		"lyrics":          "[Instrumental]",
		"instrumental":    true,
		"duration":        15.0,
		"inference_steps": 8,
		"guidance_scale":  7.0,
		"seed":            42,
		"batch_size":      1,
		"audio_format":    "flac",
		"thinking":        false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Audios   []string `json:"audios"`
		Metadata struct {
			Elapsed   float64 `json:"elapsed"`
			NumAudios int     `json:"num_audios"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(result.Audios) == 0 {
		t.Fatal("expected at least one audio in response")
	}
	if result.Audios[0] == "" {
		t.Error("first audio is empty string")
	}
	if result.Metadata.Elapsed <= 0 {
		t.Errorf("metadata.elapsed should be > 0, got %f", result.Metadata.Elapsed)
	}
	if result.Metadata.NumAudios < 1 {
		t.Errorf("metadata.num_audios should be >= 1, got %d", result.Metadata.NumAudios)
	}

	t.Logf("Generated %d audio(s) in %.2fs, first audio length: %d chars (base64)",
		result.Metadata.NumAudios, result.Metadata.Elapsed, len(result.Audios[0]))
}
