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
6. A `TelemetrySource` supplies a bounded telemetry v1 window. The current
   implementations are the ultrasonic simulator and USB-serial ESP32 reader.
7. Payload validation enforces the schema, P1-P6 identifiers, array bounds,
   finite values, confirmed profile, configured mode, and safe input metadata.
8. Signal analysis derives average/minimum/maximum voltage, variation,
   transitions, pulse count, frequency, duty cycle, jitter, missing activity,
   dropout events, shared failure buckets, stability, and baseline deviation.
9. The deterministic diagnostic engine compares those facts with component
   specifications and project baselines.
10. Structured evidence records measured, derived, specification, baseline,
   software, and future AI provenance separately.
11. A PROBE adapter may interpret that evidence and recommend a next test.
12. A test planner chooses an allowed action. Any future PATCH request passes a
   separate safety validator and user-approval boundary.
13. A new measurement window is compared with the original evidence by VERIFY.

## Replaceable boundaries

- `domain.TelemetrySource`: simulator and serial now; Wi-Fi or MQTT later.
- `domain.Repository`: SQLite now; MongoDB Atlas or another store later.
- PROBE service: deterministic mock now; Gemini adapter later.
- Vision service: Gemini image adapter when configured; explicit skipped status
  otherwise.
- Code analyzer: portable deterministic parser now; its interface permits a
  Tree-sitter adapter in a build that provides CGO and a C compiler.
- Component catalog: embedded, provenance-bearing JSON entries shared through a
  small Go module.
- Frontend API client: Go API with a browser fixture as a development fallback.

The browser fixture follows the same session response contract as the Go API. The
Go simulator and real firmware both enter the backend through the telemetry v1
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

## Trust boundaries

Device telemetry is untrusted until validated against both the telemetry schema
and the confirmed Project Profile. Uploaded source and media are bounded and
validated before persistence; source is never executed. AI output is untrusted
interpretation. PATCH is the only
component allowed to request an output signal, and its validator—not the model—has
the final decision.
