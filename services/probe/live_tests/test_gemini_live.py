"""Opt-in live test that makes a REAL call to the Gemini API.

This file lives outside services/probe/tests/ on purpose: pyproject.toml's
[tool.pytest.ini_options] testpaths = ["tests"] means the default
`uv run pytest` never collects anything here, not even as a skipped test.

To run this test explicitly, from services/probe/:

    PROBE_RUN_LIVE_GEMINI_TESTS=1 GEMINI_API_KEY=<real key> uv run pytest live_tests/ -v

Both the explicit `live_tests/` path and both env vars are required. This
test is never part of CI or the standard verification checklist.
"""

import os

import pytest

from app.config import ProbeSettings
from app.grounding import ground_evidence
from app.providers.gemini import GeminiAIProvider

pytestmark = pytest.mark.skipif(
    not os.environ.get("PROBE_RUN_LIVE_GEMINI_TESTS"),
    reason="opt-in live Gemini test; set PROBE_RUN_LIVE_GEMINI_TESTS=1 and GEMINI_API_KEY to run",
)


def test_live_intermittent_echo_against_real_gemini() -> None:
    import json
    from pathlib import Path

    fixtures_dir = Path(__file__).parent.parent / "fixtures"
    from app.schemas import StructuredEvidence

    evidence = StructuredEvidence.model_validate_json(
        (fixtures_dir / "intermittent_echo.json").read_text()
    )
    grounded = ground_evidence(evidence)

    settings = ProbeSettings(
        ai_provider="gemini",
        gemini_api_key=os.environ["GEMINI_API_KEY"],
        gemini_model=os.environ.get("GEMINI_MODEL", "gemini-flash-latest"),
        gemini_timeout_seconds=float(os.environ.get("GEMINI_TIMEOUT_SECONDS", "10")),
    )
    provider = GeminiAIProvider(settings)

    result = provider.interpret(evidence, grounded)

    print(json.dumps(result.model_dump(mode="json"), indent=2))
    assert result.outcome in ("DIAGNOSED", "UNKNOWN")
