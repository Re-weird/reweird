"""Opt-in live test that makes a REAL call to the Gemini Vision API.

Same placement/gating rules as test_gemini_code_live.py in this directory -
see that file's module docstring. Requires a real image; since none ships
with this repo (no binary test fixtures), this test builds a minimal
synthetic PNG in-memory rather than requiring a checked-in binary asset.
"""

import json
import os

import pytest

from app.catalog import get_default_catalog
from app.config import UnderstandSettings
from app.vision.provider import VisionInterpretationProvider

pytestmark = pytest.mark.skipif(
    not os.environ.get("PROBE_RUN_LIVE_GEMINI_TESTS"),
    reason="opt-in live Gemini test; set PROBE_RUN_LIVE_GEMINI_TESTS=1 and GEMINI_API_KEY to run",
)

_TINY_PNG = bytes.fromhex(
    "89504e470d0a1a0a0000000d49484452000000010000000108020000009077"
    "3df80000000a49444154789c6360000002000155a723d00000000049454e44ae426082"
)


def test_live_vision_against_real_gemini() -> None:
    settings = UnderstandSettings(
        ai_provider="gemini",
        gemini_api_key=os.environ["GEMINI_API_KEY"],
        gemini_vision_model=os.environ.get("GEMINI_VISION_MODEL", "gemini-flash-latest"),
        gemini_timeout_seconds=float(os.environ.get("GEMINI_TIMEOUT_SECONDS", "10")),
    )
    provider = VisionInterpretationProvider(settings)
    result = provider.interpret(_TINY_PNG, "image/png", get_default_catalog())

    print(json.dumps(result.model_dump(mode="json"), indent=2))
    assert result.outcome in ("INTERPRETED", "UNKNOWN")
