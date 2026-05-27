package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chasedputnam/go-music-gen/internal/schema"
)

// buildCoverRequest constructs a multipart/form-data request for POST /cover.
func buildCoverRequest(t *testing.T, fields map[string]string, audioContent []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Add reference_audio file
	fw, err := mw.CreateFormFile("reference_audio", "ref.flac")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if audioContent == nil {
		audioContent = []byte("fakeaudio")
	}
	fw.Write(audioContent)

	// Add form fields
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/cover", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestCover_HappyPath(t *testing.T) {
	h := testHandlers(t)

	req := buildCoverRequest(t, map[string]string{
		"caption":      "Jazz style cover",
		"audio_format": "flac",
		"batch_size":   "1",
	}, nil)

	rr := httptest.NewRecorder()
	h.Cover(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp schema.AudioResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Metadata.RequestType != "cover" {
		t.Errorf("request_type: got %q, want cover", resp.Metadata.RequestType)
	}
	if resp.Metadata.Elapsed <= 0 {
		t.Error("elapsed should be > 0")
	}
	if resp.Audios == nil {
		t.Error("audios should not be nil")
	}
}

func TestCover_MissingCaption(t *testing.T) {
	h := testHandlers(t)

	req := buildCoverRequest(t, map[string]string{
		"audio_format": "flac",
	}, nil)

	rr := httptest.NewRecorder()
	h.Cover(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCover_BatchSizeTooLarge(t *testing.T) {
	h := testHandlers(t)

	req := buildCoverRequest(t, map[string]string{
		"caption":    "test",
		"batch_size": "99",
	}, nil)

	rr := httptest.NewRecorder()
	h.Cover(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCover_MissingReferenceAudio(t *testing.T) {
	h := testHandlers(t)

	// Send a request without the reference_audio field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("caption", "test")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/cover", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Cover(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
