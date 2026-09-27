# Diagnostic rules

The MVP orders reasoning from strongest deterministic evidence to weaker
interpretation.

1. **Missing signal:** a profile requires activity but the window contains no
   pulse or transition evidence.
2. **Voltage outside specification:** an average measured voltage falls outside a
   trusted minimum/maximum from the confirmed profile.
3. **Unexpected dropout:** observed pulses fall below the expected rate or a
   required activity bucket is empty.
4. **Power rail instability:** configured rail variation exceeds tolerance or its
   voltage leaves the trusted range.
5. **Simultaneous dropout:** two or more probes fail in the same analysis bucket,
   suggesting a shared power, ground, or harness cause.
6. **Baseline deviation:** a derived value differs from an explicitly trusted
   healthy baseline. UNKNOWN baselines never influence a conclusion.
7. **Movement correlation:** dropout count increases by a meaningful amount and
   ratio relative to the saved pre-test window. This
   test strengthens—but does not mathematically prove—the intermittent-connection
   hypothesis.

Raw high-frequency streams are bounded at the transport and converted into
`AnalysisResult`. The optional PROBE service receives structured measurements,
derived facts, specification results, baseline comparisons, deterministic rule
results, and unresolved questions—not the raw stream.

## Per-project diagnosis (Physical Git + measurements → PROBE)

The rules above have always run inside `diagnostics.Engine`, but until now
only against the single global demo/simulator profile (`GET /api/v1/session`,
tied to one `controller.profileID` and one live `TelemetrySource`). A real
project created via `POST /api/v1/projects` had no way to ever get a live
measurement into its own `ProjectProfile`'s history, and no way to run this
engine against it at all.

Two additive endpoints close that gap without touching the rules above or the
existing global loop:

- **`POST /projects/:id/measurements`** — the missing measurement-ingestion
  path for a real project. It reuses `Engine.AnalyzeSignals` (the same
  deterministic analyzer the global loop already uses) and the existing
  idempotent, capacity-checked `SaveMeasurement` — submitting the exact same
  measurement twice returns the same stored row rather than erroring or
  duplicating it, and a `profile_id` that doesn't match the project is
  rejected outright.
- **`GET /projects/:id/diagnose`** — the per-project counterpart to
  `GET /api/v1/session`. It is never persisted separately: every call
  recomputes a `ProjectDiagnosis` from whatever is currently stored (the
  project's `ProjectProfile`, its two most recent `MeasurementWindow`s as
  current/reference, and bounded supplementary context — see below), the
  same "derive, don't cache" pattern Physical Restore and Verify already use.
  With no measurement yet, it honestly returns `INSUFFICIENT_EVIDENCE` with
  zero confidence and a concrete next step instead of fabricating a
  diagnosis.

### Physical Git context, bounded

`internal/diagnosticcontext.Build` reads a project's two most recent Physical
Commits (via the existing, deterministic `physicalgit.Diff` and its own
bounded, non-causal `DiagnosticContext` lines), the latest commit's persisted
Gemini vision interpretation, the project's analyzed software's declared pin
intent, and Component Catalog matches for components actually in its
profile — and returns at most 16 `EvidenceFact`s, merged into
`Evidence.PhysicalContext` (`Engine.AnalyzeEnvelopeWithContext`/
`DiagnoseAnalysis`). This does not grow as a project accumulates history: it
is always a small, capped slice, never the project's entire Physical Git
history.

Every fact keeps the `Provenance` that says exactly where it came from, so
PROBE (and any reader of the JSON) can never mistake one kind of fact for
another:

| Provenance | Meaning |
| --- | --- |
| `PHYSICAL_HISTORY` | A factual, non-causal observation from a `PhysicalCommitDiff` (e.g. "ECHO connection changed") — never a claim about why a fault occurred. |
| `AI_INTERPRETATION` | Gemini Vision's interpretation of a commit's saved image — explicitly never presented as measured evidence. |
| `SOFTWARE` | What the project's analyzed firmware source declares about pin usage — intent, not proof of physical wiring. |
| `SPECIFICATION` | A Component Catalog fact for a component actually present in the profile — never the whole catalog. |
| `MEASURED` / `DERIVED` / `BASELINE` | Unchanged: the engine's own deterministic rule evidence, exactly as before this section existed. |

`Engine.DiagnoseAnalysis` diagnoses an already-persisted `AnalysisResult`
directly, without requiring its original raw `TelemetryEnvelope` — this is
what lets a historical measurement (including the seeded demo project's
synthetic ones, whose `Raw.SchemaVersion` predates this build's
`signalanalysis` checks) still be diagnosed from what was already trusted and
shown at capture time.
