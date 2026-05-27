#!/usr/bin/env python3
"""
Mock inference worker for testing the Go bridge package.

Reads JSON-RPC requests from stdin and returns canned responses without
loading any ML models. Supports injecting errors and progress events via
the request's params.

Special params fields (for test control):
  _mock_error: string  -> emit a failure result with this error message
  _mock_progress: int  -> emit N progress events before the result
"""

from __future__ import annotations

import json
import sys


def emit(obj: dict) -> None:
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def handle(req: dict) -> None:
    req_id = req.get("id", "unknown")
    task_type = req.get("task_type", "text2music")
    params = req.get("params", {})

    if task_type == "preload":
        emit({"id": req_id, "success": True, "lm_available": True})
        return

    # Inject error if requested
    mock_error = params.get("_mock_error")
    if mock_error:
        emit({"id": req_id, "success": False, "error": mock_error})
        return

    # Emit progress events if requested
    mock_progress = int(params.get("_mock_progress", 0))
    for i in range(mock_progress):
        value = round((i + 1) / max(mock_progress, 1), 4)
        emit({
            "id": req_id,
            "type": "progress",
            "value": value,
            "stage": f"Mock step {i + 1}/{mock_progress}",
        })

    emit({"id": req_id, "success": True, "lm_available": True})


def main() -> None:
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError as exc:
            emit({"id": "unknown", "success": False, "error": f"JSON parse error: {exc}"})
            continue
        handle(req)


if __name__ == "__main__":
    main()
