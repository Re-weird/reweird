# Guided tests and generic VERIFY

This layer consumes a `TestRecommendation` from any provider. The current API
provides a deterministic adapter over the existing structured signal analysis;
Layer 5/PROBE may submit the same contract later. No AI output controls an
electrical pin. A recommendation sets `test_type`, `target_probes`, `reason`,
`instructions`, `duration_seconds`, `requires_user_action`, and
`requires_patch`. The planner validates every target against the confirmed,
active Project Profile. Tests are pinned to its profile ID and version.

## API flow

1. `GET /api/v1/tests/recommendation` — current deterministic recommendation.
2. `POST /api/v1/tests` — create and persist a `DiagnosticWorkflow` from a
   recommendation. A PATCH-required plan is persisted as `LOCKED` with an
   unavailable reason; no capture is allowed.
3. `POST /api/v1/tests/:id/start` — capture and store a before window. The state
   becomes `WAITING_FOR_USER` for a guided action, otherwise `READY`.
4. `POST /api/v1/tests/:id/capture` — after the user action, capture a separate
   during window, derive a `TestResult`, and persist both.
5. `POST /api/v1/tests/:id/remeasure` — after the user applies a correction,
   capture an after window and produce a generic `VerificationResult`. In the
   simulator this switches to the healthy scenario; a serial source only
   re-measures and does not modify hardware.
6. `GET /api/v1/tests/current` or `GET /api/v1/tests/:id` — restore work after
   page refresh or backend restart. `POST /api/v1/tests/:id/cancel` cancels a
   pending test.

The same workflow can evaluate movement correlation, power-rail stability,
simultaneous dropout, signal activity, trusted-baseline comparison,
frequency/timing, and re-measurement. Observations carry measured, derived,
specification, baseline, or guided-test provenance. Positive movement
correlation supports a connection-related hypothesis but does not prove a
loose wire.

VERIFY compares distinct stored windows against the profile's applicable
voltage, activity, stability, dropout, and frequency limits. Trusted baseline
deviation is included only for `USER_CONFIRMED_HEALTHY`,
`KNOWN_GOOD_CAPTURE`, or `MANUFACTURER_SPEC`; unknown baselines never establish
resolution. Outcomes are `RESOLVED`, `IMPROVED`, `UNCHANGED`, `WORSE`, and
`INCONCLUSIVE`, with metric changes and remaining issues. A healthy case that
stays healthy is `UNCHANGED`, not a fabricated repair.

## Current boundaries

- PATCH-required tests remain locked, as does `/api/v1/patch`.
- The simulator emits one deterministic 60-second raw-sample window per
  capture. The requested duration is a planning target; the UI reports the
  actual duration, and VERIFY uses that actual duration for rate comparison.
- Serial capture currently takes the next fresh telemetry envelope rather
  than aggregating many envelopes into an exact requested duration. Longer
  hardware test windows need a transport-side aggregation strategy.
- The deterministic adapter selects one next test; it does not yet prioritize
  multiple competing recommendations or draw conclusions about a physical
  repair from the test result alone.
