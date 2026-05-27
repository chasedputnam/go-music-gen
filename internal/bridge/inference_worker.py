#!/usr/bin/env python3
"""
Inference worker for the Go music generation server.

Reads newline-delimited JSON-RPC requests from stdin, runs ACE-Step inference,
emits progress events and a final result line to stdout.

Wire format:
  Request:  {"id": "...", "task_type": "...", "save_dir": "...", "params": {...}, "config": {...}}
  Progress: {"id": "...", "type": "progress", "value": 0.42, "stage": "..."}
  Result:   {"id": "...", "success": true}
           {"id": "...", "success": false, "error": "..."}

Special startup command:
  {"id": "preload", "task_type": "preload"}  -> loads models, returns result line
"""

from __future__ import annotations

import json
import logging
import os
import sys
import threading
from typing import Optional

logging.basicConfig(
    stream=sys.stderr,
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger("inference_worker")

# ---------------------------------------------------------------------------
# Project root: prefer MUSIC_PROJECT_ROOT env var (set by Go bridge when the
# script is extracted to a temp file). Fall back to two levels up from __file__
# for the case where the script is run directly from the repo.
# ---------------------------------------------------------------------------
_PROJECT_ROOT = os.getenv(
    "MUSIC_PROJECT_ROOT",
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
)

# ---------------------------------------------------------------------------
# Lazy-loaded model handles (protected by _model_lock)
# ---------------------------------------------------------------------------
_model_lock = threading.Lock()
_dit_handler = None
_llm_handler = None
_lm_available = False
_PROJECT_DEVICE = ""  # resolved after first model load
_PROJECT_DTYPE = ""   # resolved after first model load


def _emit(obj: dict) -> None:
    """Write a JSON line to stdout and flush immediately."""
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def _load_dit():
    global _dit_handler, _PROJECT_DEVICE, _PROJECT_DTYPE
    if _dit_handler is not None:
        return _dit_handler

    from acestep.handler import AceStepHandler

    dit_config = os.getenv("DIT_CONFIG", "acestep-v15-turbo")
    device = os.getenv("DEVICE", "")

    logger.info("Initializing DiT handler: %s", dit_config)
    handler = AceStepHandler()
    status_msg, ok = handler.initialize_service(
        project_root=_PROJECT_ROOT,
        config_path=dit_config,
        device=device or None,
    )
    if not ok:
        raise RuntimeError(f"DiT initialization failed: {status_msg}")
    logger.info("DiT initialized: %s", status_msg)
    _dit_handler = handler

    # Resolve device and dtype from torch after initialization
    try:
        import torch
        if device:
            _PROJECT_DEVICE = device
        elif torch.cuda.is_available():
            _PROJECT_DEVICE = "cuda"
        elif getattr(torch.backends, "mps", None) and torch.backends.mps.is_available():
            _PROJECT_DEVICE = "mps"
        elif getattr(torch.backends, "xpu", None) and torch.backends.xpu.is_available():
            _PROJECT_DEVICE = "xpu"
        else:
            _PROJECT_DEVICE = "cpu"

        precision = os.getenv("MODEL_PRECISION", "").lower()
        precision_map = {
            "float16": "float16", "fp16": "float16", "half": "float16",
            "bfloat16": "bfloat16", "bf16": "bfloat16",
            "float32": "float32", "fp32": "float32",
        }
        if precision in precision_map:
            _PROJECT_DTYPE = precision_map[precision]
        elif _PROJECT_DEVICE in {"cuda", "mps", "xpu"}:
            _PROJECT_DTYPE = "bfloat16"
        else:
            _PROJECT_DTYPE = "float32"
    except Exception:
        pass  # non-fatal — device/dtype will remain empty

    return handler


def _load_llm():
    global _llm_handler, _lm_available
    if _llm_handler is not None:
        return _llm_handler

    enable_lm = os.getenv("ENABLE_LM", "1").lower() not in {"0", "false", ""}
    if not enable_lm:
        logger.info("LM disabled by configuration")
        return None

    try:
        from acestep.llm_inference import LLMHandler

        lm_model_path = os.getenv("LM_MODEL_PATH", "acestep-5Hz-lm-1.7B")
        lm_backend = os.getenv("LM_BACKEND", "")
        device = os.getenv("DEVICE", "")
        checkpoint_dir = os.path.join(_PROJECT_ROOT, "checkpoints")

        # Auto-detect backend if not set
        if not lm_backend:
            if device == "mps":
                lm_backend = "mlx"
            else:
                lm_backend = "pt"

        logger.info("Loading LLM: %s (backend=%s)", lm_model_path, lm_backend)
        handler = LLMHandler()
        status_msg, ok = handler.initialize(
            checkpoint_dir=checkpoint_dir,
            lm_model_path=lm_model_path,
            backend=lm_backend,
            device="auto",
        )
        if not ok:
            logger.warning("LLM initialization failed: %s", status_msg)
            return None
        logger.info("LLM initialized: %s", status_msg)
        _llm_handler = handler
        _lm_available = True
        return handler
    except Exception:
        logger.warning("LLM initialization failed, running DiT-only", exc_info=True)
        return None


def _get_models():
    """Return (dit, llm) — loads lazily under lock."""
    with _model_lock:
        dit = _load_dit()
        llm = _load_llm()
    return dit, llm


def _make_progress_cb(req_id: str):
    """Return a progress callback that emits SSE-style progress lines."""
    def cb(value: float, desc: str = "") -> None:
        try:
            _emit({
                "id": req_id,
                "type": "progress",
                "value": round(float(value), 4),
                "stage": desc or "Generating...",
            })
        except Exception:
            pass  # non-fatal
    return cb


def _run_inference(req: dict) -> None:
    """Execute one inference request and emit result."""
    req_id = req.get("id", "unknown")
    task_type = req.get("task_type", "text2music")
    save_dir = req.get("save_dir", "")
    params_dict = req.get("params", {})
    config_dict = req.get("config", {})

    try:
        from acestep.inference import GenerationConfig, GenerationParams, generate_music

        dit, llm = _get_models()

        params = GenerationParams(
            caption=params_dict.get("caption", ""),
            lyrics=params_dict.get("lyrics", "[Instrumental]"),
            instrumental=params_dict.get("instrumental", False),
            vocal_language=params_dict.get("vocal_language", "en"),
            duration=params_dict.get("duration"),
            bpm=params_dict.get("bpm"),
            keyscale=params_dict.get("keyscale"),
            timesignature=params_dict.get("timesignature"),
            inference_steps=params_dict.get("inference_steps", 8),
            guidance_scale=params_dict.get("guidance_scale", 7.0),
            seed=params_dict.get("seed", -1),
            thinking=params_dict.get("thinking", True),
            task_type=task_type,
            shift=3.0,
            infer_method="ode",
            reference_audio=params_dict.get("reference_audio") or None,
            audio_cover_strength=params_dict.get("audio_cover_strength", 0.5),
            src_audio=params_dict.get("src_audio") or None,
            repainting_start=params_dict.get("repainting_start", 0.0),
            repainting_end=params_dict.get("repainting_end", 0.0),
        )

        seed = config_dict.get("seed", -1)
        cfg = GenerationConfig(
            batch_size=config_dict.get("batch_size", 1),
            audio_format=config_dict.get("audio_format", "flac"),
            use_random_seed=(seed == -1),
            seeds=None if seed == -1 else [seed],
        )

        progress_cb = _make_progress_cb(req_id)

        result = generate_music(
            dit_handler=dit,
            llm_handler=llm,
            params=params,
            config=cfg,
            save_dir=save_dir,
            progress=progress_cb,
        )

        if not result.success:
            _emit({"id": req_id, "success": False, "error": result.error or "Generation failed"})
        else:
            _emit({"id": req_id, "success": True, "lm_available": _lm_available,
                   "device": _PROJECT_DEVICE, "dtype": _PROJECT_DTYPE})

    except Exception as exc:
        logger.exception("Inference error for request %s", req_id)
        _emit({"id": req_id, "success": False, "error": str(exc)})


def _handle_preload(req: dict) -> None:
    """Preload models and emit result."""
    req_id = req.get("id", "preload")
    try:
        _get_models()
        _emit({"id": req_id, "success": True, "lm_available": _lm_available,
               "device": _PROJECT_DEVICE, "dtype": _PROJECT_DTYPE})
    except Exception as exc:
        logger.exception("Preload failed")
        _emit({"id": req_id, "success": False, "error": str(exc)})


def main() -> None:
    logger.info("inference_worker started (pid=%d)", os.getpid())

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError as exc:
            logger.error("Failed to parse request: %s", exc)
            _emit({"id": "unknown", "success": False, "error": f"JSON parse error: {exc}"})
            continue

        task_type = req.get("task_type", "")
        if task_type == "preload":
            _handle_preload(req)
        else:
            _run_inference(req)

    logger.info("inference_worker stdin closed, exiting")


if __name__ == "__main__":
    main()
