# Architecture

ReWeird follows ports-and-adapters boundaries at the points most likely to change.

## Data flow

1. A `TelemetrySource` supplies a bounded telemetry v1 window. The current
   implementations are the ultrasonic simulator and USB-serial ESP32 reader.
2. Payload validation enforces the schema, P1-P6 identifiers, array bounds,
   finite values, confirmed profile, configured mode, and safe input metadata.
3. Signal analysis derives average/minimum/maximum voltage, variation,
   transitions, pulse count, frequency, duty cycle, jitter, missing activity,
   dropout events, shared failure buckets, stability, and baseline deviation.
4. The deterministic diagnostic engine compares those facts with component
   specifications and project baselines.
5. Structured evidence records measured, derived, specification, baseline,
   software, and future AI provenance separately.
6. A PROBE adapter may interpret that evidence and recommend a next test.
7. A test planner chooses an allowed action. Any future PATCH request passes a
   separate safety validator and user-approval boundary.
8. A new measurement window is compared with the original evidence by VERIFY.

## Replaceable boundaries

- `domain.TelemetrySource`: simulator and serial now; Wi-Fi or MQTT later.
- `domain.Repository`: SQLite now; MongoDB Atlas or another store later.
- PROBE service: deterministic mock now; Gemini adapter later.
- Vision service: placeholder now; Gemini vision adapter later.
- Frontend API client: Go API with a browser fixture as a development fallback.

The browser fixture follows the same session response contract as the Go API. The
Go simulator and real firmware both enter the backend through the telemetry v1
contract and the same signal analyzer. The fixture is not a second product
architecture; it keeps the hackathon UI runnable when Go or hardware is absent.

## Project Profiles

Profiles are persisted JSON documents behind the repository interface. A profile
owns controller and logic-voltage context, component specifications, probe roles,
expected signals, safe measurement scale, optional baseline, and user-confirmed
status. The engine has no built-in meaning for P1, P2, P3, ECHO, TRIG, 5.01 V, or
40 kHz; those values come from the active profile and current telemetry.

## Trust boundaries

Device telemetry is untrusted until validated against both the telemetry schema
and the confirmed Project Profile. Uploaded source and media are
untrusted until scanned. AI output is untrusted interpretation. PATCH is the only
component allowed to request an output signal, and its validator—not the model—has
the final decision.
