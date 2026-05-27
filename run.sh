#!/bin/bash

set -euo pipefail

cd "$(dirname "$0")"

BINARY="./go-music-gen"

# Build if binary doesn't exist or source is newer
if [[ ! -f "$BINARY" ]] || find . -name "*.go" -newer "$BINARY" | grep -q .; then
  echo "Building go-music-gen..."
  go build -o "$BINARY" ./cmd/server/
fi

HOST=${HOST:-0.0.0.0}
PORT=${PORT:-4009}
MUSIC_ENV=${MUSIC_ENV:-production}

PASS_MODE_FLAG=""
for arg in "$@"; do
  case "$arg" in
    --dev)
      MUSIC_ENV=development
      PASS_MODE_FLAG="--dev"
      ;;
    --prod)
      MUSIC_ENV=production
      PASS_MODE_FLAG="--prod"
      ;;
  esac
done

export MUSIC_ENV

# ---------------------------------------------------------------------------
# Resolve Python binary
# Priority:
#   1. PYTHON_BIN env var (explicit override)
#   2. .python-path file written by setup.sh
#   3. Sibling music-gen.server venv (auto-detect)
#   4. System python3 / python (fallback)
# ---------------------------------------------------------------------------
PYTHON_BIN="${PYTHON_BIN:-}"

if [[ -z "$PYTHON_BIN" && -f ".python-path" ]]; then
  PYTHON_BIN="$(cat .python-path)"
fi

if [[ -z "$PYTHON_BIN" ]]; then
  SIBLING_VENV="$(cd "$(dirname "$0")/../music-gen.server" 2>/dev/null && pwd || true)/.venv/bin/python"
  if [[ -f "$SIBLING_VENV" ]]; then
    PYTHON_BIN="$SIBLING_VENV"
  fi
fi

if [[ -z "$PYTHON_BIN" ]]; then
  for bin in python3 python; do
    if command -v "$bin" &>/dev/null; then
      PYTHON_BIN="$(command -v "$bin")"
      break
    fi
  done
fi

if [[ -z "$PYTHON_BIN" ]]; then
  echo "Error: no Python interpreter found." >&2
  echo "Run ./setup.sh first, or set PYTHON_BIN=/path/to/python." >&2
  exit 1
fi

echo "Starting Kortexa Music Generation Server (Go) ($MUSIC_ENV)..."
echo "Python: $PYTHON_BIN"
exec "$BINARY" --host "$HOST" --port "$PORT" --python "$PYTHON_BIN" $PASS_MODE_FLAG
