package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// parseSSEEvents reads all SSE events from a response body and returns them as
// a slice of {event, data} pairs.
func parseSSEEvents(body string) []map[string]string {
	var events []map[string]string
	current := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(current) > 0 {
				events = append(events, current)
				current = map[string]string{}
			}
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			current["event"] = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			current["data"] = strings.TrimPrefix(line, "data: ")
		}
	}
	if len(current) > 0 {
		events = append(events, current)
	}
	return events
}

func postJSONStream(t *testing.T, h *Handlers, path string, body any, query string) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	url := path
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.GenerateStream(rr, req)
	return rr
}

func TestGenerateStream_EventSequence(t *testing.T) {
	h := testHandlers(t)

	rr := postJSONStream(t, h, "/generate/stream", map[string]any{
		"caption":         "A lo-fi beat",
		"duration":        15.0,
		"inference_steps": 8,
		"guidance_scale":  7.0,
		"seed":            42,
		"instrumental":    true,
		"audio_format":    "flac",
		"batch_size":      1,
	}, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type: got %q, want text/event-stream", ct)
	}
	if rr.Header().Get("Cache-Control") != "no-cache" {
		t.Error("missing Cache-Control: no-cache")
	}
	if rr.Header().Get("X-Accel-Buffering") != "no" {
		t.Error("missing X-Accel-Buffering: no")
	}

	events := parseSSEEvents(rr.Body.String())
	if len(events) == 0 {
		t.Fatal("no SSE events received")
	}

	// Last two events must be metadata then done
	n := len(events)
	if n < 2 {
		t.Fatalf("expected at least 2 events (metadata + done), got %d", n)
	}
	if events[n-2]["event"] != "metadata" {
		t.Errorf("second-to-last event: got %q, want metadata", events[n-2]["event"])
	}
	if events[n-1]["event"] != "done" {
		t.Errorf("last event: got %q, want done", events[n-1]["event"])
	}

	// Verify metadata parses correctly
	var meta map[string]any
	if err := json.Unmarshal([]byte(events[n-2]["data"]), &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if meta["request_type"] != "text2music" {
		t.Errorf("metadata.request_type: got %v", meta["request_type"])
	}
}

func TestGenerateStream_IncludeAudioFalse(t *testing.T) {
	h := testHandlers(t)

	rr := postJSONStream(t, h, "/generate/stream", map[string]any{
		"caption":      "test",
		"audio_format": "flac",
	}, "include_audio=false")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	events := parseSSEEvents(rr.Body.String())
	for _, e := range events {
		if e["event"] == "audio_chunk" {
			t.Error("audio_chunk event should not be present when include_audio=false")
		}
	}

	// Must still have metadata and done
	eventNames := make([]string, len(events))
	for i, e := range events {
		eventNames[i] = e["event"]
	}
	hasMetadata, hasDone := false, false
	for _, name := range eventNames {
		if name == "metadata" {
			hasMetadata = true
		}
		if name == "done" {
			hasDone = true
		}
	}
	if !hasMetadata {
		t.Error("missing metadata event")
	}
	if !hasDone {
		t.Error("missing done event")
	}
}

func TestGenerateStream_ValidationError(t *testing.T) {
	h := testHandlers(t)

	rr := postJSONStream(t, h, "/generate/stream", map[string]any{
		"caption": "",
	}, "")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGenerateStream_WithMockProgress(t *testing.T) {
	// Use a mock worker that emits progress events.
	// We can't inject _mock_progress through the normal handler path,
	// so we verify the stream handler correctly forwards whatever progress
	// the bridge sends. With the standard mock worker (no _mock_progress),
	// we just verify no panic and correct event sequence.
	h := testHandlers(t)

	rr := postJSONStream(t, h, "/generate/stream", map[string]any{
		"caption":      "test progress",
		"audio_format": "flac",
	}, "")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	events := parseSSEEvents(rr.Body.String())
	n := len(events)
	if n < 2 {
		t.Fatalf("expected at least metadata+done, got %d events", n)
	}
	if events[n-1]["event"] != "done" {
		t.Errorf("last event should be done, got %q", events[n-1]["event"])
	}
}
