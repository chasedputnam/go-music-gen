package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/chasedputnam/go-music-gen/internal/bridge"
	"github.com/chasedputnam/go-music-gen/internal/schema"
)

const sseAudioChunkSize = 256 * 1024 // 256 KB of base64 string data

// writeSSE writes a single SSE event and flushes the response writer.
func writeSSE(w http.ResponseWriter, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

// GenerateStream handles POST /generate/stream (SSE text-to-music).
func (h *Handlers) GenerateStream(w http.ResponseWriter, r *http.Request) {
	var req schema.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	schema.ApplyDefaults(&req, h.Cfg)

	if err := schema.Validate(&req, h.Cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Parse include_audio query param (default true)
	includeAudio := r.URL.Query().Get("include_audio") != "false"

	log.Printf("generate/stream: caption=%q duration=%v steps=%d guidance=%.1f",
		truncate(req.Caption, 80), req.Duration, req.InferenceSteps, req.GuidanceScale)

	// Set SSE headers before writing anything
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	saveDir, cleanup, err := h.makeTempDir("music_gen_sse_")
	if err != nil {
		writeSSE(w, "error", map[string]string{"detail": "failed to create temp dir"})
		return
	}
	defer cleanup()

	progressCh := make(chan bridge.ProgressEvent, 32)
	resultCh := make(chan struct {
		resp *bridge.InferenceResponse
		err  error
	}, 1)

	ireq := bridge.InferenceRequest{
		ID:       newRequestID(),
		TaskType: "text2music",
		SaveDir:  saveDir,
		Params: bridge.InferenceParams{
			Caption:        req.Caption,
			Lyrics:         req.Lyrics,
			Instrumental:   req.Instrumental,
			VocalLanguage:  req.VocalLanguage,
			Duration:       req.Duration,
			BPM:            req.BPM,
			KeyScale:       req.KeyScale,
			TimeSignature:  req.TimeSignature,
			InferenceSteps: req.InferenceSteps,
			GuidanceScale:  req.GuidanceScale,
			Seed:           *req.Seed,
			Thinking:       req.Thinking,
		},
		Config: bridge.InferenceConfig{
			BatchSize:   req.BatchSize,
			AudioFormat: req.AudioFormat,
			Seed:        *req.Seed,
		},
	}

	start := time.Now()

	// Run inference in a goroutine
	go func() {
		resp, err := h.Bridge.Generate(r.Context(), ireq, progressCh)
		close(progressCh)
		resultCh <- struct {
			resp *bridge.InferenceResponse
			err  error
		}{resp, err}
	}()

	// Drain progress events until the goroutine closes progressCh
	for evt := range progressCh {
		if err := writeSSE(w, "progress", map[string]any{
			"value": evt.Value,
			"stage": evt.Stage,
		}); err != nil {
			log.Printf("generate/stream: client disconnected during progress")
			return
		}
		// Check if client disconnected
		select {
		case <-r.Context().Done():
			log.Printf("generate/stream: client disconnected")
			return
		default:
		}
	}

	// Get result
	res := <-resultCh
	if res.err != nil {
		if res.err == context.Canceled {
			log.Printf("generate/stream: cancelled")
			return
		}
		log.Printf("generate/stream: bridge error: %v", res.err)
		writeSSE(w, "error", map[string]string{"detail": "Music generation failed"})
		return
	}
	if !res.resp.Success {
		log.Printf("generate/stream: inference error: %s", res.resp.Error)
		writeSSE(w, "error", map[string]string{"detail": res.resp.Error})
		return
	}

	elapsed := time.Since(start).Seconds()
	numAudios := 0

	if includeAudio {
		audios := collectAudioFiles(saveDir, req.AudioFormat)
		numAudios = len(audios)
		for audioIdx, audioB64 := range audios {
			totalChunks := (len(audioB64) + sseAudioChunkSize - 1) / sseAudioChunkSize
			for chunkIdx := 0; chunkIdx < totalChunks; chunkIdx++ {
				lo := chunkIdx * sseAudioChunkSize
				hi := lo + sseAudioChunkSize
				if hi > len(audioB64) {
					hi = len(audioB64)
				}
				if err := writeSSE(w, "audio_chunk", map[string]any{
					"audio_index":  audioIdx,
					"chunk_index":  chunkIdx,
					"total_chunks": totalChunks,
					"data":         audioB64[lo:hi],
				}); err != nil {
					log.Printf("generate/stream: write audio_chunk error: %v", err)
					return
				}
			}
		}
	}

	// Metadata event
	writeSSE(w, "metadata", schema.InferenceMetadata{
		RequestType:   "text2music",
		DitConfig:     h.Cfg.DitConfig,
		Device:        h.Cfg.Device,
		Dtype:         h.Cfg.ModelPrecision,
		Caption:       req.Caption,
		Duration:      req.Duration,
		Steps:         req.InferenceSteps,
		GuidanceScale: req.GuidanceScale,
		Seed:          *req.Seed,
		Elapsed:       elapsed,
		NumAudios:     numAudios,
		AudioFormat:   req.AudioFormat,
		LMEnabled:     h.Bridge.LMAvailable(),
	})

	// Done event
	writeSSE(w, "done", map[string]any{
		"elapsed":    math.Round(elapsed*100) / 100,
		"num_audios": numAudios,
	})

	log.Printf("generate/stream: done %d audios in %.2fs", numAudios, elapsed)
}
