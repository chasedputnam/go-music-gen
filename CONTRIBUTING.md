# Contributing to go-music-gen

Thanks for your interest in contributing! This guide covers everything you need to get started.

## Ways to Contribute

- **Report bugs** — open a [Bug Report](https://github.com/chasedputnam/go-music-gen/issues/new?template=bug_report.yml)
- **Request features** — open a [Feature Request](https://github.com/chasedputnam/go-music-gen/issues/new?template=feature_request.yml)
- **Submit pull requests** — bug fixes, new features, documentation improvements
- **Improve docs** — fix typos, clarify setup steps, add examples

## Development Setup

### Prerequisites

- Go 1.21+
- Python 3.12+ with ACE-Step installed (same virtualenv as the Python server)
- ACE-Step model checkpoints in `checkpoints/` (downloaded by `setup.sh`)
- A supported compute device: CUDA GPU, Apple Silicon (MPS), Intel GPU (XPU), or CPU

### Getting Started

```bash
git clone https://github.com/chasedputnam/go-music-gen.git
cd go-music-gen

# Download Go dependencies
go mod download

# Build the server binary
go build -o go-music-gen ./cmd/server

# Run tests (no GPU or model weights required — uses mock_worker.py)
go test ./...
```

### Project Structure

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

### Build Commands

```bash
# Build server binary
go build -o go-music-gen ./cmd/server

# Run all tests (uses mock worker, no GPU needed)
go test ./...

# Run tests with race detection
go test -race ./...

# Run linter
go vet ./...

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy

# End-to-end test against a live server
MUSIC_ENDPOINT=http://localhost:4009/generate go test ./tests/ -v
```

## Making Changes

1. **Fork** the repository and create a branch from `main`:
   ```bash
   git checkout -b fix/describe-your-change
   ```

2. **Make your changes.** Keep commits focused — one logical change per commit.

3. **Run tests and lint** before pushing:
   ```bash
   go vet ./...
   go test -race ./...
   go mod tidy
   ```

4. **Open a pull request** against `main`. Fill out the PR template — describe what changed and how you tested it.

## Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Bug fix | `fix/short-description` | `fix/sse-stream-flush` |
| Feature | `feat/short-description` | `feat/batch-generate` |
| Docs | `docs/short-description` | `docs/cover-endpoint-example` |
| Chore | `chore/short-description` | `chore/update-chi` |

## Code Conventions

- Follow standard Go formatting — run `gofmt` before committing
- Keep package responsibilities focused: `bridge/` for subprocess/JSON-RPC, `handler/` for HTTP, `schema/` for validation — don't cross boundaries
- Add or update tests for any logic changes; test files live alongside source files (`*_test.go`)
- The Python worker is embedded via `//go:embed` in `internal/bridge/` — keep `python/inference_worker.py` as the source of truth and sync the embedded copy
- Avoid adding new dependencies unless necessary; discuss in an issue first
- All temp directories must be registered with `tmpdir.Registry` for leak-free cleanup on shutdown

## Commit Messages

Use the [Conventional Commits](https://www.conventionalcommits.org/) style:

```
feat: add webhook callback support for async generation
fix: flush SSE stream on each progress event
docs: add repaint endpoint example to README
chore: bump chi to v5.2.5
test: add schema validation edge cases for duration bounds
```

## Security Issues

Do **not** open a public issue for security vulnerabilities. See [SECURITY.md](SECURITY.md) for the private reporting process.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
