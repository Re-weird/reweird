# Architecture

ReWeird follows ports-and-adapters boundaries at the points most likely to change.

## Data flow

1. The user creates a persisted `Project`, uploads a validated PNG/JPEG and
   uploads or pastes bounded UTF-8 source code.
2. Deterministic code analysis extracts GPIO constants, modes, reads/writes,
   pulses, PWM, I2C roles, libraries, and timing without compiling or executing
   the source.
3. The optional Gemini Vision adapter returns component and relationship
   suggestions labeled `VISION_AI`. With no key, the same analysis completes as
   `VISION_SKIPPED` using code, user input, and the catalog.
4. Project Understanding merges user input, static facts, vision suggestions,
   and catalog specifications into one draft `ProjectProfile`. Conflicting
   sources remain side by side until the user resolves them.
5. User confirmation persists an immutable authoritative profile and generates
   generic GND/P1-P6 placement instructions. Probe connection confirmation is a
   separate persisted action.
6. A `TelemetrySource` supplies a bounded telemetry v2 window. Every frame names
   the confirmed Project Profile it was configured for. The current
   implementations are the ultrasonic simulator and USB-serial ESP32 reader.
7. Payload validation enforces the schema, P1-P6 identifiers, array bounds,
   finite values, confirmed profile, configured mode, and safe input metadata.
8. Accepted raw frames and their normalized analysis are stored together as
   immutable measurement windows in SQLite.
9. Signal analysis derives average/minimum/maximum voltage, variation,
   transitions, pulse count, frequency, duty cycle, pulse-width statistics,
   jitter, maximum gap, missing activity, dropout events, shared failure
   buckets, rail stability, and baseline deviation.
10. The deterministic diagnostic engine compares those facts with component
   specifications and project baselines.
11. Structured evidence records measured, derived, specification, baseline,
   software, and future AI provenance separately.
12. The Go PROBE adapter submits that evidence to the Python intelligence
   service, which validates grounding references, produces evidence-backed
   hypotheses, and deterministically recommends a next test. UNKNOWN or any
   service failure preserves the original Go diagnosis unchanged.
13. Main's persisted test planner chooses an allowed action from that
   recommendation. Any future PATCH request passes a
   separate safety validator and user-approval boundary.
14. A new measurement window is compared with the original evidence by VERIFY.

## Replaceable boundaries

- `domain.TelemetrySource`: simulator and serial now; Wi-Fi or MQTT later.
- `domain.Repository`: SQLite now; MongoDB Atlas or another store later.
- PROBE service: the deterministic Go engine always computes the fail-closed
  diagnosis; `internal/probe.ServiceProvider` sends only `domain.Evidence` and
  that diagnosis to Python `services/probe`. Python owns grounding, optional
  Gemini interpretation, and its Test Planner, then maps back to main's existing
  `Diagnosis` contract. No service URL, UNKNOWN, malformed output, or request
  failure leaves the deterministic wording unchanged.
- Vision service: Gemini image adapter when configured; explicit skipped status
  otherwise.
- Code analyzer: portable deterministic parser now; its interface permits a
  Tree-sitter adapter in a build that provides CGO and a C compiler.
- Component catalog: embedded, provenance-bearing JSON entries shared through a
  small Go module.
- Frontend API client: Go API with a browser fixture as a development fallback.

The browser fixture follows the same session response contract as the Go API. The
Go simulator and real firmware both enter the backend through the telemetry v2
contract and the same signal analyzer. The fixture is not a second product
architecture; it keeps the hackathon UI runnable when Go or hardware is absent.

## Project Profiles

Projects and profiles are separate persisted records. A project can exist while
uploads are incomplete or before analysis. Its draft profile records controller
and logic voltage, catalog-enriched components, proposed connections, per-field
evidence, conflicts, unresolved questions, and confirmation status. Confirmation
generates probe configurations and a placement plan; it never upgrades AI output
into measured or specification evidence. The engine has no built-in meaning for
P1, P2, P3, ECHO, TRIG, 5.01 V, or 40 kHz.

Draft analysis and profile confirmation write the project and profile together
in one SQLite transaction. The built-in ultrasonic simulator remains isolated to
its demo profile; a non-demo profile waits for matching real telemetry.
Generic profile writes are draft-only and cannot modify a project-backed or
confirmed profile. Confirmation metadata and probe assignments are created by
the project confirmation endpoint after conflict checks; confirmed profiles
cannot be silently replaced by a new upload or analysis.

The browser circuit map is a projection of profile components/connections and
the generated probe plan. It never infers a physical wire from a profile edge.
Probe status is overlaid only from a stored capture whose profile, raw frame,
and analysis IDs match the displayed confirmed profile (and whose non-demo
project probe plan has been marked connected). This is an observation, not
visual wiring verification or a component-health verdict.

Known Good records are immutable references to persisted measurement windows.
The server derives PHYSICAL versus SIMULATED from the stored telemetry source,
checks profile/device/raw/analysis identity and required probe stability, and
requires an explicit local user healthy report. It never accepts a client-
claimed source or converts simulator evidence into a physical baseline. Physical
records additionally require a confirmed project probe plan. Baselines are
applied to a copy of the confirmed profile only for the same source and device
at analysis time, preserving the confirmed profile. The Device Passport joins
this record with the profile/map, recent captures, guided diagnostic history,
and source-labeled resolved VERIFY outcomes following user-reported actions. Its status uses
measured deviation/verification labels, not a fabricated health percentage.
Because ESP32 frames currently have no trusted wall clock, measurement order
and display time use the backend's persisted ingestion time where needed.
Repeated identical frames reuse their immutable measurement row; a reused
source/device/sequence/time identity with different raw or derived evidence is rejected
instead of mutating baseline evidence. Device boot identity remains a hardware
validation task.

## Trust boundaries

Device telemetry is untrusted until validated against both the telemetry schema
and the confirmed Project Profile. Uploaded source and media are bounded and
validated before persistence; source is never executed. AI output is untrusted
interpretation. PATCH is the only
component allowed to request an output signal, and its validator—not the model—has
the final decision.
