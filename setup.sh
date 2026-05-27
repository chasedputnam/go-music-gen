#!/bin/bash

set -euo pipefail

# ---------------------------------------------------------------------------
# go-music-gen setup
#
# This script sets up the Python environment and downloads ACE-Step model
# checkpoints required by the Go server's embedded inference worker.
#
# The Python virtualenv and checkpoints are managed in the sibling
# music-gen.server repo. If that repo is not present, this script will
# clone it and run its setup.sh instead.
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PYTHON_REPO_DIR="$(cd "$SCRIPT_DIR/../music-gen.server" 2>/dev/null && pwd)" || true

# ---------------------------------------------------------------------------
# 1. Ensure the Python repo exists and is set up
# ---------------------------------------------------------------------------
if [[ -z "$PYTHON_REPO_DIR" || ! -d "$PYTHON_REPO_DIR" ]]; then
    echo "music-gen.server not found at ../music-gen.server"
    echo ""
    echo "Cloning music-gen.server..."
    git clone https://github.com/chasedputnam/music-gen.server.git "$SCRIPT_DIR/../music-gen.server"
    PYTHON_REPO_DIR="$(cd "$SCRIPT_DIR/../music-gen.server" && pwd)"
fi

if [[ ! -d "$PYTHON_REPO_DIR/.venv" ]]; then
    echo "Python virtualenv not found. Running music-gen.server/setup.sh..."
    echo ""
    bash "$PYTHON_REPO_DIR/setup.sh"
else
    echo "Found existing virtualenv at $PYTHON_REPO_DIR/.venv"

    # Still ensure models are present even if venv already exists
    echo ""
    echo "Checking ACE-Step model checkpoints..."
    cd "$PYTHON_REPO_DIR"
    .venv/bin/python -c "
from pathlib import Path
from acestep.model_downloader import ensure_main_model, ensure_dit_model
cp = Path('checkpoints')
cp.mkdir(exist_ok=True)
ok, msg = ensure_main_model(cp, prefer_source='huggingface')
print(msg)
ok2, msg2 = ensure_dit_model('acestep-v15-turbo', cp, prefer_source='huggingface')
print(msg2)
" 2>/dev/null || {
        echo "Note: Could not verify model checkpoints (ACE-Step may not be installed yet)."
        echo "Run music-gen.server/setup.sh to install dependencies and download models."
    }
    cd "$SCRIPT_DIR"
fi

# ---------------------------------------------------------------------------
# 2. Verify the Python binary is accessible
# ---------------------------------------------------------------------------
PYTHON_BIN="$PYTHON_REPO_DIR/.venv/bin/python"
if [[ ! -f "$PYTHON_BIN" ]]; then
    echo ""
    echo "Error: Python binary not found at $PYTHON_BIN"
    echo "Run music-gen.server/setup.sh to set up the virtualenv."
    exit 1
fi
echo ""
echo "Python: $($PYTHON_BIN --version)"

# Write .python-path so run.sh can find the venv Python without activation
echo "$PYTHON_BIN" > "$SCRIPT_DIR/.python-path"
echo "Wrote .python-path -> $PYTHON_BIN"

# ---------------------------------------------------------------------------
# 3. Build the Go binary
# ---------------------------------------------------------------------------
if ! command -v go &> /dev/null; then
    cat <<'MSG'

Error: Go is not installed.

Install Go via one of:
  brew install go
  https://go.dev/dl/

MSG
    exit 1
fi

echo ""
echo "Building go-music-gen..."
cd "$SCRIPT_DIR"
go build -o go-music-gen ./cmd/server/
echo "Built: $SCRIPT_DIR/go-music-gen"

# ---------------------------------------------------------------------------
# 4. Done
# ---------------------------------------------------------------------------
echo ""
echo "Setup complete."
echo ""
echo "Run: ./run.sh                    # Starts server on port 4009 (prod)"
echo "     ./run.sh --dev              # Development mode (no model preload)"
echo "     ./go-music-gen --help       # All CLI flags"
echo ""
echo "The server uses the Python virtualenv at:"
echo "  $PYTHON_REPO_DIR/.venv"
echo ""
echo "Model checkpoints are at:"
echo "  $PYTHON_REPO_DIR/checkpoints/"
