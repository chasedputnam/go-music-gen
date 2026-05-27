# go-music-gen

A Go HTTP server that exposes [ACE-Step](https://github.com/ACE-Step/ACE-Step-1.5) diffusion model inference as a synchronous REST API. It is the Go successor to [`music-gen.server`](https://github.com/kortexa-ai/music-gen.server) (Python/FastAPI), delivering better runtime performance, lower memory overhead, and a single-binary deployment model consistent with a Go-based services architecture.

## What is this?

go-music-gen is a thin HTTP wrapper around ACE-Step 1.5, a state-of-the-art music generation diffusion model. It provides:

- **Synchronous generation:** `POST /generate` returns base64-encoded audio directly in the response — no task IDs, no polling, no separate download step.
- **SSE streaming:** `POST /generate/stream` streams real-time progress events and audio chunks as the model runs, so clients can show a progress bar without waiting for the full response.
- **Style transfer:** `POST /cover` applies a style caption to a reference audio file using ACE-Step's cover task.
- **Region editing:** `POST /repaint` regenerates a specific time range within an existing audio file.
- **Single binary:** The Go binary embeds the Python inference worker via `//go:embed`. No separate script files needed at runtime.
- **Wire-compatible:** All endpoints, request/response shapes, SSE event names, and field names are identical to the Python server. Existing clients require no changes.

## Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│  go-music-gen (Go binary)                                    │
├──────────────────────────────────────────────────────────────┤
│  ► HTTP Router (chi)  ─► /health, /generate, /cover, ...    │
│  ► Request Validation ─► All field constraints enforced      │
│  ► SSE Streaming      ─► Progress + chunked audio events     │
│  ► Bridge             ─► JSON-RPC over stdin/stdout          │
│  ► Temp Dir Registry  ─► Leak-free cleanup on shutdown       │
└──────────────────────────────────────────────────────────────┘
                               │
                    JSON-RPC (stdin/stdout)
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│  inference_worker.py (Python subprocess, embedded)           │
├──────────────────────────────────────────────────────────────┤
│  ► AceStepHandler  ─► DiT diffusion model (required)        │
│  ► LLMHandler      ─► Language model for lyrics (optional)  │
└──────────────────────────────────────────────────────────────┘
```

The Go binary spawns one Python subprocess on startup and keeps it alive for the lifetime of the server. A single mutex serializes all inference calls — GPU memory cannot support parallel ACE-Step runs. If the subprocess crashes, the bridge attempts one automatic restart before surfacing a 500 error.

---

## Prerequisites

- **Go 1.21+**
- **Python 3.12+** with ACE-Step installed (same virtualenv as the Python server)
- **ACE-Step model checkpoints** in `checkpoints/` (downloaded by `setup.sh` from the Python repo)
- A machine with a supported compute device: CUDA GPU, Apple Silicon (MPS), Intel GPU (XPU), or CPU

---

## Setup

### 1. Download models

Run the setup script from the Python repo once to create the virtualenv and download ACE-Step checkpoints:

```bash
cd ~/repo/music-gen.server
./setup.sh
```

This downloads the DiT model (`acestep-v15-turbo`) and the LLM (`acestep-5Hz-lm-1.7B`) into `checkpoints/`.

### 2. Build

```bash
cd ~/repo/go-music-gen
go build -o go-music-gen ./cmd/server/
```

The binary embeds `inference_worker.py` at build time. No Python files need to be present at runtime.

### 3. Run

```bash
# Production mode (models preloaded on startup)
./run.sh

# Development mode (preload skipped, faster restart)
./run.sh --dev

# Custom host and port
HOST=127.0.0.1 PORT=8080 ./run.sh

# Direct binary invocation
./go-music-gen --host 0.0.0.0 --port 4009 --prod
```

The server prints a startup banner and begins accepting requests. In production mode, model loading happens before the first request is served (typically 30–120 seconds depending on hardware). In development mode, models load lazily on the first request.

---

## Configuration

All settings are controlled via environment variables. CLI flags (`--host`, `--port`, `--dev`, `--prod`) override the corresponding env vars.

| Variable | Default | Description |
|---|---|---|
| `HOST` | `0.0.0.0` | Bind address |
| `PORT` | `4009` | Listen port |
| `MUSIC_ENV` | `production` | Runtime mode (`development` or `production`) |
| `DIT_CONFIG` | `acestep-v15-turbo` | ACE-Step DiT model config name |
| `LM_MODEL_PATH` | `acestep-5Hz-lm-1.7B` | LLM model path for lyric-aware generation |
| `ENABLE_LM` | `1` | Enable LLM (`0` or `false` to run DiT-only) |
| `LM_BACKEND` | auto | LM inference backend (`pt` for CUDA, `mlx` for Apple Silicon) |
| `DEVICE` | auto | Compute device (`cuda`, `mps`, `xpu`, `cpu`) |
| `MODEL_PRECISION` | auto | Torch dtype (`float16`, `bfloat16`, `float32`) |
| `PRELOAD_MODELS` | `1` in prod, `0` in dev | Load models before accepting requests |
| `DEFAULT_DURATION` | `30` | Default generation duration in seconds |
| `MAX_DURATION` | `600` | Maximum allowed duration |
| `NUM_INFERENCE_STEPS` | `8` | Default number of diffusion steps |
| `GUIDANCE_SCALE` | `7.0` | Default classifier-free guidance scale |
| `AUDIO_FORMAT` | `flac` | Default output audio format |
| `MAX_BATCH_SIZE` | `2` | Maximum allowed batch size per request |

**Auto-detection:** When `DEVICE` is not set, the Python worker detects the best available device at model load time (`cuda` → `mps` → `xpu` → `cpu`). When `MODEL_PRECISION` is not set, it defaults to `bfloat16` on GPU/MPS/XPU and `float32` on CPU. The resolved values are reported in `GET /health` after the first preload or inference call.

---

## API

All endpoints are wire-compatible with the Python `music-gen.server`. Existing clients targeting port 4009 require no changes.

### `GET /health`

Returns server status and resolved runtime configuration.

```json
{
  "status": "ok",
  "device": "mps",
  "dtype": "bfloat16",
  "dit_config": "acestep-v15-turbo",
  "lm_model": "acestep-5Hz-lm-1.7B",
  "lm_enabled": true,
  "lm_backend": "mlx",
  "lm_available": true
}
```

`device` and `dtype` reflect the values resolved by the Python worker after model load. They are empty strings until the first preload or inference call completes.

---

### `POST /generate`

Synchronous text-to-music generation. Returns base64-encoded audio directly in the response.

**Request body (JSON):**

| Field | Type | Default | Description |
|---|---|---|---|
| `caption` | string | _(required)_ | Style/genre description, 1–512 chars |
| `lyrics` | string | `[Instrumental]` | Song lyrics, max 4096 chars |
| `instrumental` | bool | `false` | Skip lyrics entirely |
| `vocal_language` | string | `en` | Language code for vocals |
| `duration` | float | `30` | Duration in seconds (10–600) |
| `bpm` | int | `null` | Beats per minute (30–300) |
| `keyscale` | string | `null` | Musical key (e.g. `C major`) |
| `timesignature` | string | `null` | Time signature (`2/4`, `3/4`, `4/4`, `6/8`) |
| `inference_steps` | int | `8` | Diffusion steps (1–100) |
| `guidance_scale` | float | `7.0` | Guidance scale (0.0–20.0) |
| `seed` | int | `-1` | Random seed (-1 for random, 0–4294967295 for reproducible) |
| `batch_size` | int | `1` | Number of audio files to generate (1–`MAX_BATCH_SIZE`) |
| `audio_format` | string | `flac` | Output format (`mp3`, `wav`, `flac`, `wav32`, `opus`, `aac`) |
| `thinking` | bool | `false` | Enable ACE-Step chain-of-thought mode |

**Response:**

```json
{
  "audios": ["<base64-encoded audio>", "..."],
  "metadata": {
    "request_type": "text2music",
    "dit_config": "acestep-v15-turbo",
    "device": "mps",
    "dtype": "bfloat16",
    "caption": "A gentle lo-fi hip hop beat",
    "duration": 30,
    "steps": 8,
    "guidance_scale": 7.0,
    "seed": -1,
    "elapsed": 18.43,
    "num_audios": 1,
    "audio_format": "flac",
    "lm_enabled": true
  }
}
```

**Example:**

```bash
curl -s -X POST http://localhost:4009/generate \
  -H "Content-Type: application/json" \
  -d '{
    "caption": "A gentle lo-fi hip hop beat with warm piano chords and vinyl crackle",
    "instrumental": true,
    "duration": 15,
    "inference_steps": 8,
    "seed": 42,
    "audio_format": "flac"
  }' | jq -r '.audios[0]' | base64 -d > output.flac
```

---

### `POST /generate/stream`

SSE streaming variant of `/generate`. Returns `Content-Type: text/event-stream`. Same request body as `/generate`.

**Query parameters:**

| Parameter | Default | Description |
|---|---|---|
| `include_audio` | `true` | Set to `false` to receive only progress and metadata events without audio chunks |

**Event sequence:**

```
event: progress
data: {"value": 0.12, "stage": "Thinking..."}

event: progress
data: {"value": 0.45, "stage": "Denoising step 4/8"}

event: audio_chunk
data: {"audio_index": 0, "chunk_index": 0, "total_chunks": 3, "data": "<base64 chunk ≤256KB>"}

event: audio_chunk
data: {"audio_index": 0, "chunk_index": 1, "total_chunks": 3, "data": "<base64 chunk>"}

event: metadata
data: {"request_type": "text2music", "elapsed": 18.43, ...}

event: done
data: {"elapsed": 18.43, "num_audios": 1}
```

If generation fails, an `event: error` is emitted and the stream closes:

```
event: error
data: {"detail": "Music generation failed"}
```

---

### `POST /cover`

Style transfer using a reference audio file. Accepts `multipart/form-data`.

**Form fields:**

| Field | Type | Default | Description |
|---|---|---|---|
| `reference_audio` | file | _(required)_ | Reference audio file (any common format) |
| `caption` | string | _(required)_ | Target style description |
| `audio_cover_strength` | float | `0.5` | Blend strength (0.0–1.0) |
| _(all other `/generate` fields)_ | | | Same defaults and constraints |

**Example:**

```bash
curl -s -X POST http://localhost:4009/cover \
  -F "reference_audio=@my_track.flac" \
  -F "caption=Jazz piano trio, upbeat swing" \
  -F "audio_cover_strength=0.6" \
  -F "audio_format=flac" \
  | jq -r '.audios[0]' | base64 -d > cover.flac
```

---

### `POST /repaint`

Regenerates a specific time region of an existing audio file. Accepts `multipart/form-data`.

**Form fields:**

| Field | Type | Default | Description |
|---|---|---|---|
| `src_audio` | file | _(required)_ | Source audio file to edit |
| `caption` | string | _(required)_ | Style description for the repainted region |
| `repainting_start` | float | _(required)_ | Start of region in seconds |
| `repainting_end` | float | _(required)_ | End of region in seconds (must be > start) |
| _(all other `/generate` fields)_ | | | Same defaults and constraints |

**Example:**

```bash
curl -s -X POST http://localhost:4009/repaint \
  -F "src_audio=@my_track.flac" \
  -F "caption=Brighter, more energetic chorus" \
  -F "repainting_start=32.0" \
  -F "repainting_end=64.0" \
  -F "audio_format=flac" \
  | jq -r '.audios[0]' | base64 -d > repainted.flac
```

---

## First Start Behaviour

On a fresh start in production mode:

1. The Go binary starts and spawns the Python inference worker subprocess.
2. It sends a `preload` command to the worker over JSON-RPC.
3. The worker loads the DiT model (~10–60s depending on hardware) and optionally the LLM (~30–120s).
4. Once preload completes, the HTTP server begins accepting requests.
5. The startup banner logs the resolved device, dtype, and elapsed preload time.

In development mode (`--dev`), preload is skipped and models load lazily on the first request. This makes restarts faster but means the first request will be slow.

**If the LLM fails to load**, the server continues in DiT-only mode and logs a warning. Lyric-aware generation will still work but without the LLM's chain-of-thought enhancement.

---

## Development and Testing

```bash
# Run all unit and integration tests (no GPU or model weights required)
go test ./...

# Run with verbose output
go test ./... -v

# End-to-end test against a live server with real models
MUSIC_ENDPOINT=http://localhost:4009/generate go test ./tests/ -v

# Build
go build -o go-music-gen ./cmd/server/

# Format and tidy
go fmt ./...
go mod tidy
```

Tests use a `mock_worker.py` that returns canned responses without loading any ML models, so the full test suite runs in CI without a GPU.

---

## Project Layout

```
go-music-gen/
├── cmd/server/main.go              # CLI entrypoint, flag parsing, startup banner, graceful shutdown
├── internal/
│   ├── bridge/
│   │   ├── bridge.go               # Subprocess lifecycle, JSON-RPC transport, restart policy
│   │   ├── inference_worker.py     # Embedded Python sidecar (go:embed)
│   │   └── types.go                # Wire types: InferenceRequest, InferenceResponse, ProgressEvent
│   ├── config/config.go            # Settings struct, env var resolution with defaults
│   ├── handler/
│   │   ├── generate.go             # POST /generate
│   │   ├── stream.go               # POST /generate/stream (SSE)
│   │   ├── cover.go                # POST /cover (multipart)
│   │   ├── repaint.go              # POST /repaint (multipart)
│   │   ├── health.go               # GET /health
│   │   ├── audio.go                # collectAudioFiles + base64 encoding
│   │   ├── handler.go              # Handlers struct, shared helpers
│   │   └── util.go                 # saveUpload, makeTempDir, truncate, newRequestID
│   ├── middleware/logging.go       # Request logging middleware
│   ├── schema/schema.go            # GenerateRequest, AudioResponse, Validate, ApplyDefaults
│   └── tmpdir/registry.go          # Shutdown-safe temp directory registry
├── python/
│   ├── inference_worker.py         # ACE-Step JSON-RPC worker (source of truth for embed)
│   └── mock_worker.py              # Test mock — returns canned responses, no ML models
├── tests/e2e_test.go               # End-to-end test (requires MUSIC_ENDPOINT + live server)
├── run.sh                          # Convenience start script with auto-build
├── go.mod
└── go.sum
```

---

## Relation to music-gen.server

This project is a drop-in replacement for [`music-gen.server`](https://github.com/chasedputnam/music-gen.server). The API surface is identical — same endpoints, same JSON field names, same SSE event names, same default port (4009). The key differences:

| | music-gen.server (Python) | go-music-gen (Go) |
|---|---|---|
| HTTP framework | FastAPI + uvicorn | net/http + chi |
| Inference | In-process (Python) | Subprocess JSON-RPC |
| Binary | Python package | Single Go binary |
| Model loading | uvicorn worker process | Embedded Python sidecar |
| Deployment | `uv run` / virtualenv | `./go-music-gen` |
| Dev reload | `--reload` flag (uvicorn) | `--dev` flag (no preload) |
