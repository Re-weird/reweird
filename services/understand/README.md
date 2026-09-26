# Project Understanding service

This service answers a question that comes *before* PROBE's diagnostics can
run at all: what does a user's electronics project actually contain, and
what is it meant to do? It never diagnoses a fault - it proposes a starting
point for a human to confirm.

## Pipeline

1. **Tree-sitter code analysis** (`app/code_analysis/extractor.py`) parses
   the user's own C/C++ (Arduino/PlatformIO) source and extracts literal,
   deterministic `CodeFact`s: pin constants, `pinMode`/`digitalWrite`/
   `analogRead`-family calls, `#include` directives, and symbol-to-pin
   bindings. It never infers a component, a role, or a diagnostic
   conclusion - only what is literally present in the parsed syntax tree.
   Malformed code still yields best-effort facts from parseable regions
   (`parse_errors` records what didn't parse); an unsupported language
   yields zero facts without crashing.

2. **Gemini code interpretation** (`app/code_analysis/provider.py`) reasons
   only over those facts plus a closed index of `packages/component-catalog/`
   entries, proposing likely component matches and pin roles - each one
   `AI_INTERPRETATION` provenance, each one required to cite real `ref_id`s
   and real catalog IDs. A deterministic preflight skips the model call
   entirely when there are no facts to interpret. Output is validated
   **all-or-nothing**: any single hallucinated reference invalidates the
   entire response back to `UNKNOWN`, never a partially-trusted result.

3. **Gemini vision** (`app/vision/provider.py`), optional, does the same for
   a photo. Its output schema has no field capable of expressing a numeric
   measurement at all - a structural guarantee, not a prompt-level request,
   that vision can never claim an exact electrical reading. Code-only
   analysis always works completely without an image; a vision failure
   never blocks the code path.

4. **Proposal assembly** (`app/proposal.py`) merges both modalities. When
   they agree, results are combined with `sources: ["code_analysis",
   "vision"]`. When they name different, non-overlapping catalog components
   for the same project, nothing is silently chosen - `status` becomes
   `PROPOSED_WITH_CONFLICTS`, both candidates are kept, and an
   `unresolved_questions` entry names the disagreement for a human to
   resolve.

## Why this never touches `ProjectProfile` directly

`ProjectProfile.probes[].probe` (`P1`-`P6`) names one of *ReWeird's own*
physical measurement-harness channels - which channel gets clipped onto
which point on the target board is a decision only a human makes, later,
with the device in hand. No amount of code or image analysis can determine
it. So `ProjectProfileProposal` deliberately has no `probe` field anywhere;
`ProposedRole.target_pin` names the *project's own* pin (e.g. `"GPIO25"`)
instead. Turning an accepted proposal into a real, submittable
`ProjectProfile` needs exactly one more piece of information a human
supplies - the `target_pin -> P1..P6` mapping - after which the existing
`POST/PUT /api/v1/profiles` (Go API, unchanged) is used as-is, with
`confirmed: true` set by whoever is doing the confirming. This service never
calls that endpoint and never sets `confirmed: true` anywhere.

## Configuration

    UNDERSTAND_AI_PROVIDER=fake   # or "gemini"
    GEMINI_API_KEY=
    GEMINI_MODEL=gemini-flash-latest
    GEMINI_VISION_MODEL=gemini-flash-latest
    GEMINI_TIMEOUT_SECONDS=10

Under the default `fake` provider, deterministic tree-sitter extraction
still runs in full; only the AI-interpretation layer becomes a no-op
(`UNKNOWN`/`PROVIDER_UNCERTAIN`), so the service boots and serves requests
with zero configuration. Providers are constructed once at startup and
cached - a missing `GEMINI_API_KEY` fails fast at boot when
`UNDERSTAND_AI_PROVIDER=gemini`, not on a caller's first request.

## Run

    uv sync
    uv run pytest -v
    uv run uvicorn app.api:app --reload --port 8092

Tests never make a live Gemini call and never require a real API key. Two
opt-in live tests exist outside the default suite, in `live_tests/`:

    PROBE_RUN_LIVE_GEMINI_TESTS=1 GEMINI_API_KEY=<real key> uv run pytest live_tests/ -v

## Try it

    curl -s -X POST http://localhost:8092/understand \
      -H "content-type: application/json" \
      -d @- <<'EOF' | python -m json.tool
    {"files": [{"path": "main.cpp", "content": "#define TRIG_PIN 25\nvoid setup() { pinMode(TRIG_PIN, OUTPUT); }"}]}
    EOF
