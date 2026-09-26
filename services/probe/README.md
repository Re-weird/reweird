# PROBE service

PROBE is the deterministic AI-diagnostic orchestrator for ReWeird. Its input is a
structured evidence object (`StructuredEvidence` — measurements, derived facts,
specification results, baseline comparisons, deterministic rule results, and
unresolved questions; never the raw telemetry stream). Its output is ranked
hypotheses, each grounded in specific evidence reference IDs so nothing is
hallucinated, plus a deterministically recommended next test.

Milestone 1 runs entirely without any network or AI calls: `FakeAIProvider` is a
rule-based, deterministic interpreter of the same rule vocabulary used by the Go
diagnostic engine (`missing-signal`, `voltage-outside-specification`,
`power-rail-instability`, `unexpected-dropout`, `simultaneous-dropout`,
`baseline-deviation`, `movement-correlation`). When evidence doesn't support a
confident conclusion (no rule results, or contradictory rule results), PROBE
returns an explicit `UNKNOWN` outcome instead of guessing.

`AIProvider` is a small interface (`app/providers/base.py`) so a future
`GeminiAIProvider` can be swapped in later purely via the FastAPI dependency
injection point in `app/api.py` — no change to the orchestration, grounding, or
test-planner code. Milestone 1 intentionally ships with no Gemini/network calls.

PROBE never writes measurements or addresses hardware directly. It has no
`/patch` route and never will — hardware control is a separate, permanently
locked boundary in the Go API (`HTTP 423`).

## Run

    uv sync
    uv run pytest -v
    uv run uvicorn app.api:app --reload --port 8091

## Try it

    curl -s -X POST http://localhost:8091/probe \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/intermittent_echo.json)}" | python -m json.tool
