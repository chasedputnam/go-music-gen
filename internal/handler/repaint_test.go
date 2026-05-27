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

func buildRepaintRequest(t *testing.T, fields map[string]string, audioContent []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile("src_audio", "src.flac")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if audioContent == nil {
		audioContent = []byte("fakeaudio")
	}
	fw.Write(audioContent)

	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/repaint", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestRepaint_HappyPath(t *testing.T) {
	h := testHandlers(t)

	req := buildRepaintRequest(t, map[string]string{
		"caption":          "Brighter chorus",
		"repainting_start": "10.0",
		"repainting_end":   "20.0",
		"audio_format":     "flac",
		"batch_size":       "1",
	}, nil)

	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp schema.AudioResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Metadata.RequestType != "repaint" {
		t.Errorf("request_type: got %q, want repaint", resp.Metadata.RequestType)
	}
	if resp.Metadata.Elapsed <= 0 {
		t.Error("elapsed should be > 0")
	}
	if resp.Audios == nil {
		t.Error("audios should not be nil")
	}
}

func TestRepaint_InvalidRange(t *testing.T) {
	h := testHandlers(t)

	// end <= start
	req := buildRepaintRequest(t, map[string]string{
		"caption":          "test",
		"repainting_start": "20.0",
		"repainting_end":   "10.0",
	}, nil)

	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRepaint_EqualRange(t *testing.T) {
	h := testHandlers(t)

	req := buildRepaintRequest(t, map[string]string{
		"caption":          "test",
		"repainting_start": "10.0",
		"repainting_end":   "10.0",
	}, nil)

	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for equal range, got %d", rr.Code)
	}
}

func TestRepaint_BatchSizeTooLarge(t *testing.T) {
	h := testHandlers(t)

	req := buildRepaintRequest(t, map[string]string{
		"caption":          "test",
		"repainting_start": "0.0",
		"repainting_end":   "10.0",
		"batch_size":       "99",
	}, nil)

	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRepaint_MissingSrcAudio(t *testing.T) {
	h := testHandlers(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("caption", "test")
	mw.WriteField("repainting_start", "0.0")
	mw.WriteField("repainting_end", "10.0")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/repaint", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRepaint_MissingCaption(t *testing.T) {
	h := testHandlers(t)

	req := buildRepaintRequest(t, map[string]string{
		"repainting_start": "0.0",
		"repainting_end":   "10.0",
	}, nil)

	rr := httptest.NewRecorder()
	h.Repaint(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
