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

`AIProvider` is a small interface (`app/providers/base.py`) implemented by
both `FakeAIProvider` (Milestone 1, still the default) and `GeminiAIProvider`
(Milestone 2) — swapped in purely via the FastAPI dependency injection point
in `app/api.py`, with no change to the orchestration, grounding, or
test-planner code either way.

PROBE never writes measurements or addresses hardware directly. It has no
`/patch` route and never will — hardware control is a separate, permanently
locked boundary in the Go API (`HTTP 423`).

## Gemini provider (Milestone 2)

`GeminiAIProvider` reasons only over the same `StructuredEvidence` plus the
deterministic evidence references (`GroundedItem`s) Milestone 1 already
computes — never raw telemetry, and it cannot create measurements: its output
schema has no field capable of expressing one.

Before any network call, a deterministic **preflight**
(`app/providers/preflight.py`) runs the same evidentiary checks
`FakeAIProvider` applies: no rule results → `UNKNOWN`/`NO_EVIDENCE`;
conflicting rule statuses → `UNKNOWN`/`CONFLICTING_RULES`; and an
UNKNOWN/untrusted baseline has its `baseline_comparison` facts,
`baseline-deviation` rule, and even the numeric contents of the `baseline`
mapping itself stripped before the model ever sees them (only `status` is
kept). If nothing usable survives, PROBE returns `UNKNOWN` without calling
Gemini at all.

The model's own output is a closed, bounded JSON schema
(`app/providers/gemini_schema.py`: `extra="forbid"`, length/count/range
bounds on every field) and is validated **all-or-nothing**
(`app/providers/gemini_validation.py`): every hypothesis must cite only
`ref_id`s and rule IDs that were actually given to it, and every claimed
`supporting_rule_ids` entry must have a matching cited `rule_result`
reference. If a single hypothesis fails any check, timeout, or the response
fails to parse at all, the **entire** result becomes `UNKNOWN`
(`INVALID_PROVIDER_OUTPUT` or `PROVIDER_ERROR`) — never a partially-trusted
mix. `recommended_test` is still always produced by the unchanged
`TestPlanner`, never by Gemini.

Configuration (env vars, see `.env.example`):

    PROBE_AI_PROVIDER=fake   # or "gemini"
    GEMINI_API_KEY=
    GEMINI_MODEL=gemini-flash-latest
    GEMINI_TIMEOUT_SECONDS=10

The provider is constructed once at process startup (and cached) so a
missing `GEMINI_API_KEY` fails fast at boot when `PROBE_AI_PROVIDER=gemini`,
rather than on a caller's first request.

## Component Intelligence (Milestone 4)

`packages/component-catalog/` (read-only, `app/catalog.py`) can now supply a
**deterministic expected specification** for a named component role -
supply-voltage range, pulse-width range, and whether a signal is required -
each numeric value traced to a cited datasheet source
(`packages/component-catalog/README.md`). `POST /probe` accepts one new,
**optional** field, `component_id` (e.g. `"hc-sr04"`); every existing
Milestone 1/2 request that omits it behaves exactly as before (regression-
tested in `tests/test_probe_component.py`).

When `component_id` is given, `app/specification.py`'s pure, deterministic
functions compare the catalog's specification against the submitted
evidence's own typed measurements (never the untyped `observed` dict, which
has no unit) and merge the result into `specification_results`/`rule_results`
**before** grounding or any Gemini call - `ground_evidence`, preflight's
conflict detection, and Gemini's all-or-nothing validation all already
generically support `SPECIFICATION`-provenance facts and needed zero changes.

Two new rule ids were added to the existing rule vocabulary, and no more:
`pulse-width-outside-specification` (a new checkable dimension) and
`specification-not-evaluable` - a single WARN id covering every "Python
cannot deterministically decide this" case (no catalog spec for this role,
no matching observed measurement, or a missing/mismatched unit). This is
deliberately never confused with `missing-signal`: that fail-status id is
only ever emitted when a *present* measurement deterministically shows zero
activity for a required signal. Lacking a measurement is not evidence of
absence, so it is never reported as one.

An unknown `component_id` resolves to `UNKNOWN`/`UNKNOWN_COMPONENT` before
any provider is called at all - proven with a call-counting fake provider in
tests. Gemini (or `FakeAIProvider`) can only explain/rank an already-computed
rule result; nothing in `Hypothesis` can mutate a `RuleResult`'s status, so a
provider cannot override a deterministic finding even if it tries.

## Run

    uv sync
    uv run pytest -v
    uv run uvicorn app.api:app --reload --port 8091

Tests never make a live Gemini call and never require a real API key. An
opt-in live test exists outside the default suite, in `live_tests/`:

    PROBE_RUN_LIVE_GEMINI_TESTS=1 GEMINI_API_KEY=<real key> uv run pytest live_tests/ -v

## Try it

    curl -s -X POST http://localhost:8091/probe \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/intermittent_echo.json)}" | python -m json.tool

    # With a catalog-derived specification check:
    curl -s -X POST http://localhost:8091/probe \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/hc_sr04_voltage_outside_spec.json), \"component_id\": \"hc-sr04\"}" \
      | python -m json.tool
