"""Opt-in live test that makes a REAL call to the Gemini API.

Lives outside services/understand/tests/ on purpose: pyproject.toml's
[tool.pytest.ini_options] testpaths = ["tests"] means the default
`uv run pytest` never collects anything here.

To run explicitly, from services/understand/:

    PROBE_RUN_LIVE_GEMINI_TESTS=1 GEMINI_API_KEY=<real key> uv run pytest live_tests/ -v
"""

import json
import os
from pathlib import Path

import pytest

from app.catalog import get_default_catalog
from app.code_analysis.extractor import analyze_files
from app.code_analysis.provider import CodeInterpretationProvider
from app.config import UnderstandSettings

pytestmark = pytest.mark.skipif(
    not os.environ.get("PROBE_RUN_LIVE_GEMINI_TESTS"),
    reason="opt-in live Gemini test; set PROBE_RUN_LIVE_GEMINI_TESTS=1 and GEMINI_API_KEY to run",
)


def test_live_healthy_ultrasonic_against_real_gemini() -> None:
    fixtures_dir = Path(__file__).parent.parent / "fixtures" / "code"
    source = (fixtures_dir / "healthy_ultrasonic.cpp").read_bytes()
    result = analyze_files({"healthy_ultrasonic.cpp": source})

    settings = UnderstandSettings(
        ai_provider="gemini",
        gemini_api_key=os.environ["GEMINI_API_KEY"],
        gemini_model=os.environ.get("GEMINI_MODEL", "gemini-flash-latest"),
        gemini_timeout_seconds=float(os.environ.get("GEMINI_TIMEOUT_SECONDS", "10")),
    )
    provider = CodeInterpretationProvider(settings)
    interpretation = provider.interpret(result.facts, get_default_catalog())

    print(
        json.dumps(
            {
                "outcome": interpretation.outcome,
                "components": [c.model_dump(mode="json") for c in interpretation.components],
                "roles": [r.model_dump(mode="json") for r in interpretation.roles],
            },
            indent=2,
        )
    )
    assert interpretation.outcome in ("INTERPRETED", "UNKNOWN")
