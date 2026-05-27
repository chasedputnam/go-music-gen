# Changelog

All notable changes to go-music-gen are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

---

## [1.0.0] - 2026-05-28

### Added
- HTTP server wrapping ACE-Step 1.5 diffusion model inference via embedded Python sidecar
- `POST /generate` — synchronous text-to-music generation returning base64-encoded audio
- `POST /generate/stream` — SSE streaming variant with real-time progress and chunked audio events
- `POST /cover` — style transfer using a reference audio file (`multipart/form-data`)
- `POST /repaint` — region editing to regenerate a specific time range within an existing audio file
- `GET /health` — server status and resolved runtime configuration (device, dtype, model names)
- JSON-RPC bridge over stdin/stdout for subprocess communication (`internal/bridge`)
- Automatic subprocess restart on crash (one retry before surfacing 500)
- Single mutex serializing all inference calls — safe for single-GPU deployments
- Request validation with field constraints enforced before reaching the worker (`internal/schema`)
- Environment variable configuration with sensible defaults (`internal/config`)
- `--dev` / `--prod` CLI flags and `MUSIC_ENV` env var for lazy vs. eager model loading
- `--host` and `--port` CLI flags with `HOST` / `PORT` env var overrides
- Shutdown-safe temp directory registry — all upload dirs cleaned on graceful shutdown (`internal/tmpdir`)
- Request logging middleware (`internal/middleware`)
- `run.sh` convenience script with auto-build and flag passthrough
- Full unit test suite across all packages using `mock_worker.py` (no GPU required)
- End-to-end test against a live server (`tests/e2e_test.go`, requires `MUSIC_ENDPOINT`)
- Wire-compatible with [`music-gen.server`](https://github.com/chasedputnam/music-gen.server) — same endpoints, field names, SSE event names, and default port (4009)

### Configuration

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

---

[Unreleased]: https://github.com/chasedputnam/go-music-gen/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/chasedputnam/go-music-gen/releases/tag/v1.0.0
