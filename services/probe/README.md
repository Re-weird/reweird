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

## Closed-loop diagnostic sessions (Milestone 5)

`POST /probe` is still a one-shot call. `app/sessions.py` adds an optional,
**in-memory-only** layer on top of it (no database) that turns repeated
`run_probe` calls into an auditable diagnostic loop:

    POST /sessions                     create a session from initial evidence
    POST /sessions/{id}/evidence       submit REAL new StructuredEvidence
    GET  /sessions/{id}                the complete, ordered step history
    POST /sessions/{id}/stop           human-initiated termination

Every `DiagnosticStep` is append-only - submitting step 2 never mutates step
1 - and stores both the caller's raw `submitted_evidence` and the
post-Milestone-4-merge `evaluated_evidence`, so the deterministic
specification/rule state behind any hypothesis is always auditable later.
`component_id` is fixed for the life of a session and Milestone 4's
catalog/specification evaluation reruns, unchanged, on every step.

**Previous hypotheses are never evidence.** A step's `AI_INTERPRETATION`
result is never fed back into the next step's `StructuredEvidence`, never
promoted to `MEASURED`/`DERIVED`/`SPECIFICATION`/`BASELINE`/`SOFTWARE`
provenance, and never available for a later hypothesis to cite as grounding
- each step's `ground_evidence()` call only ever sees that step's own
freshly-submitted facts. `TestPlanner` (unchanged, still not Gemini) still
decides `recommended_test` - an instruction for what to measure next, never
a claim that the test already happened.

Two safeguards prevent a session from running forever or faking progress:
- **A bounded step count** (`max_steps`, default 10): once reached, the
  session is forced to `STOPPED`/`max_steps_reached` regardless of outcome.
- **Deterministic duplicate detection**: exact-duplicate evidence (a stable
  hash of its canonical JSON, checked against every prior step, not just the
  latest) never appends a new step, changes status, or increases confidence
  - `POST /sessions/{id}/evidence` returns `duplicate: true` and the step
  number it matches instead.

Session status is deliberately conservative and independent of how
confident any hypothesis sounds: `DIAGNOSED` only when the outcome is
grounded *and* every rule for that step passed (the genuine no-fault case);
a real fail/warn rule keeps the session `ACTIVE` even though PROBE's
per-call `outcome` already says `DIAGNOSED`, because a meaningful next test
still exists. `UNKNOWN` means the provider/evidence couldn't support a safe
conclusion this round; `STOPPED` means a human or the step limit ended it -
once `STOPPED`, new evidence is rejected (`409`) but history is never
deleted.

Storage is a small `SessionStore` protocol (`app/sessions.py`) behind a
plain in-process `InMemorySessionStore` - deterministic, dependency-free,
and swappable later without touching the API layer.

## PATCH proposal + VERIFY intelligence (Milestone 6)

PROBE can now *propose* a temporary diagnostic patch and *verify* its real-
world effect - but it never executes anything. Go's `PATCH_LOCKED` gate
(`apps/api/internal/httpapi/server.go`) remains the sole, unmodified
authority for actual hardware execution; nothing here is named `/patch` for
exactly that reason:

    POST /sessions/{id}/patch-proposals                       propose a patch
    GET  /sessions/{id}/patch-proposals                       list proposals
    GET  /sessions/{id}/patch-proposals/{pid}                 get one
    POST /sessions/{id}/patch-proposals/{pid}/result          record an
                                                               EXTERNAL result
    POST /sessions/{id}/patch-proposals/{pid}/verify          verify with
                                                               REAL new evidence

**A `PatchProposal` is only a proposal** - never a GPIO command, firmware,
serial data, or evidence that a patch happened. `PatchType` is a closed
`Literal` with exactly one member in this milestone
(`TEMPORARY_SIGNAL_EMULATION`) - not a generic hardware-control language.
`app/patch_provider.py`'s `FakePatchProposalProvider` (the only provider
wired in by default - narrow and network-free, deliberately not a full
Gemini integration for this milestone) drafts `purpose`/`expected_effect`
text and *selects* which already-grounded evidence justifies the patch;
`app/patch_proposals.py.create_patch_proposal()` then re-validates
everything regardless of which provider produced it - generate-then-
validate, exactly like every other AI-touched path in this service:

- every `evidence_refs` entry must exist in the source step's own grounded
  evidence (a hallucinated ref invalidates the whole proposal);
- at least one cited ref must actually pertain to `target_probe`;
- free text is scanned for anything resembling an executable hardware
  instruction (`digitalWrite(`, `GPIO.`, `Serial.write(`, ...) or a claim
  that execution already happened - a defense-in-depth check, since the
  primary guarantee is structural (no field on `PatchProposal` can express
  either one in the first place).

**External execution is operational metadata, never evidence.**
`POST .../result` moves a proposal through `PROPOSED -> APPROVED_EXTERNALLY
-> EXECUTED_EXTERNALLY` (or `REJECTED`) - a plain, validated state machine.
`"executed"` only records that some external subsystem *reports* the patch
was applied; it is never merged into any `StructuredEvidence` and never
treated as a measurement.

**VERIFY requires real, new `StructuredEvidence`** - `{"patch_worked": true}`
is rejected by the schema itself (422), not by convention. Verifying reuses
Milestone 5's `submit_evidence()` unmodified (append-only history, Milestone
4's catalog/spec re-evaluation, duplicate detection all apply exactly as
before), then `app/verification.py`'s pure functions deterministically
compare the session's own focus probe's rule status **before** the patch
(the step that justified the proposal) against **after** (the newly
appended step):

- `SUPPORTED`: every rule that was failing before is passing after, matching
  the proposal's predicted effect.
- `NOT_SUPPORTED`: a predicted rule is still failing after.
- `INCONCLUSIVE`: the after-evidence lacks a comparable measurement for some
  predicted rule (never silently treated as SUPPORTED), or there was nothing
  failing before to verify against in the first place.

An `INCONCLUSIVE` result is deliberately **not terminal** - the proposal
stays `EXECUTED_EXTERNALLY` so a real retry with better evidence is still
possible; `SUPPORTED`/`NOT_SUPPORTED` move it to `VERIFIED`/
`FAILED_VERIFICATION`, after which re-verifying is rejected (`409`).
Only rule-status transitions (already unit-safe, computed once by
Milestone 4) decide the outcome; a secondary, exact-unit-only fact-value
diff is included for transparency but never drives the determination
itself - no unit conversion, no Gemini arithmetic where Python can compare
values directly.

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

    # Closed-loop session:
    SESSION=$(curl -s -X POST http://localhost:8091/sessions \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/hc_sr04_not_evaluable.json), \"component_id\": \"hc-sr04\"}")
    SESSION_ID=$(echo "$SESSION" | python -c "import sys,json;print(json.load(sys.stdin)['session_id'])")
    curl -s -X POST "http://localhost:8091/sessions/$SESSION_ID/evidence" \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/hc_sr04_missing_echo_activity.json)}" | python -m json.tool
    curl -s "http://localhost:8091/sessions/$SESSION_ID" | python -m json.tool

    # PATCH proposal + VERIFY (using the TRIG+ECHO fixture instead):
    SESSION2=$(curl -s -X POST http://localhost:8091/sessions \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/hc_sr04_trig_and_echo_missing.json), \"component_id\": \"hc-sr04\"}")
    SESSION2_ID=$(echo "$SESSION2" | python -c "import sys,json;print(json.load(sys.stdin)['session_id'])")
    PROPOSAL=$(curl -s -X POST "http://localhost:8091/sessions/$SESSION2_ID/patch-proposals" \
      -H "content-type: application/json" \
      -d '{"target_probe": "P2", "target_role": "TRIG", "patch_type": "TEMPORARY_SIGNAL_EMULATION"}')
    PROPOSAL_ID=$(echo "$PROPOSAL" | python -c "import sys,json;print(json.load(sys.stdin)['proposal']['proposal_id'])")
    curl -s -X POST "http://localhost:8091/sessions/$SESSION2_ID/patch-proposals/$PROPOSAL_ID/result" \
      -H "content-type: application/json" -d '{"external_status": "APPROVED_EXTERNALLY"}' > /dev/null
    curl -s -X POST "http://localhost:8091/sessions/$SESSION2_ID/patch-proposals/$PROPOSAL_ID/result" \
      -H "content-type: application/json" -d '{"external_status": "EXECUTED_EXTERNALLY"}' > /dev/null
    curl -s -X POST "http://localhost:8091/sessions/$SESSION2_ID/patch-proposals/$PROPOSAL_ID/verify" \
      -H "content-type: application/json" \
      -d "{\"evidence\": $(cat fixtures/hc_sr04_healthy.json)}" | python -m json.tool

    # History + export (Milestone 7):
    curl -s http://localhost:8091/sessions | python -m json.tool
    curl -s "http://localhost:8091/sessions/$SESSION2_ID/export" | python -m json.tool
    curl -s http://localhost:8091/health | python -m json.tool

## History persistence + sponsor integrations (Milestone 7)

`app/repository.py`'s `DiagnosticRepository` protocol replaces M5/M6's
separate in-memory stores with one storage seam - `InMemorySessionStore`/
`InMemoryPatchProposalStore` remain available and unchanged underneath it.
**PROBE's diagnostic correctness never depends on any of the systems below**;
each one only stores, indexes, or analyzes an already-computed result.
Nothing here is Intelligence Layer runtime code - a developer can clone this
repo and run the full test suite with zero network access, using the safe
defaults (`DIAGNOSTIC_REPOSITORY=memory`, `TELEMETRY_SINK=none`,
`ANALYTICS_SINK=none`).

| System | Role | Status |
|---|---|---|
| **Gemini** | AI reasoning/explanation for hypotheses and patch-proposal drafting (Milestones 2, 6) | **IMPLEMENTED** (default: `fake`, network-free; real client behind `PROBE_AI_PROVIDER=gemini` + `GEMINI_API_KEY`) |
| **MongoDB Atlas** | Persistent diagnostic session/history documents (`sessions`, `patch_proposals`, `events` collections) | **IMPLEMENTED**, **OPTIONAL** (default: `memory`; real adapter behind `DIAGNOSTIC_REPOSITORY=mongodb` + `MONGODB_URI`) |
| **Tiger Data** (PostgreSQL-compatible) | Time-series electrical measurement storage, separate from session documents | **IMPLEMENTED**, **OPTIONAL** (default: `none`; real adapter behind `TELEMETRY_SINK=tiger` + `TIGER_DATABASE_URL`) |
| **Snowflake** | Optional diagnostic analytics/export summaries | **ADAPTER BOUNDARY ONLY** - the real `snowflake-connector-python` SDK is deliberately not vendored (large transitive dependency tree); `ANALYTICS_SINK=snowflake` fails clearly at startup unless that package is installed separately. **CONFIGURED** requires `SNOWFLAKE_ACCOUNT`/`_USER`/`_PASSWORD`/`_DATABASE`/`_WAREHOUSE`; **NOT CONFIGURED** by default |
| **DigitalOcean** | Hosting/deployment target | **NOT DEPLOYED** - `Dockerfile`/`.dockerignore` here make the service deployment-ready (see below); no DigitalOcean API call exists anywhere in this service |
| **GoDaddy** | Domain/product registration | **NOT IMPLEMENTED** here, deliberately - domain management is not an Intelligence Layer runtime feature |

None of this ever promotes stored data back into trusted evidence: every
repository/sink method only ever accepts an already-computed
`DiagnosticSession`/`PatchProposal`/measurement and stores or forwards it
unchanged. Loading a database record back as NEW input context is a
possible future feature, deliberately out of scope here.

### Repository (MongoDB Atlas)

`DIAGNOSTIC_REPOSITORY=memory|mongodb` (default `memory`). Selecting
`mongodb` without `MONGODB_URI` set fails clearly at startup (`RuntimeError`,
surfaced by the FastAPI `lifespan` hook), never silently falling back to
memory. A repository failure during a request (connection lost, write
error) is a **PRIMARY** failure and is surfaced to the caller as `503`,
unlike the sinks below.

### Telemetry sink (Tiger Data)

`TELEMETRY_SINK=none|tiger` (default `none`). Extracts only numeric,
already-provenanced `EvidenceFact`s (`MEASURED`/`DERIVED`/`SPECIFICATION`/
`BASELINE`/`SOFTWARE`) from each step's evaluated evidence -
`extract_telemetry_records` never reads `rule_results` or any `Hypothesis`,
and `record_measurement` refuses (`ValueError`) anything carrying
`AI_INTERPRETATION` provenance as a defense-in-depth guard. A sink failure
is logged and swallowed at the API layer - it is optional history, and an
infrastructure outage must never become diagnostic evidence or alter the
actual response.

### Analytics sink (Snowflake)

`ANALYTICS_SINK=none|snowflake` (default `none`). `build_session_summary()`
derives only small, safe aggregate fields (step count, final status, rule
failure count, whether a patch was proposed, verification outcome,
duration) - never raw evidence, source code, or images. Like the telemetry
sink, a failure here is logged and swallowed, never allowed to alter the
deterministic diagnosis.

### Export

`GET /sessions/{id}/export` returns the full ordered diagnostic timeline
(evidence, deterministic rules/specification results, hypotheses,
recommended tests) plus PATCH proposals and VerificationResults, with
provenance intact - and a defensive `_redact_secrets` pass that replaces any
dict key matching `api_key|secret|password|token|credential|authorization`
before the export is ever returned, even though none of the underlying
models carry such a field today.

### DigitalOcean deployment readiness

    docker build -f services/probe/Dockerfile -t reweird-probe .   # from the repo root
    docker run --rm -p 8091:8091 reweird-probe

The image boots correctly with zero configuration (`fake`/`memory`/`none`
defaults baked in as safe `ENV` values, never a secret), exposes `GET
/health`, and does not touch the root `docker-compose.yml` (owned by another
subsystem) - this is a standalone, service-local deployment artifact.
