# Architecture

ReWeird follows ports-and-adapters boundaries at the points most likely to change.

## Data flow

1. A `TelemetrySource` supplies normalized probe measurements.
2. Signal analysis converts sample windows into small facts such as dropout count,
   average voltage, frequency, and stability.
3. The deterministic diagnostic engine compares those facts with component
   specifications and project baselines.
4. Structured evidence records expected, observed, baseline, and rule results.
5. A PROBE adapter may interpret that evidence and recommend a next test.
6. A test planner chooses an allowed action. Any future PATCH request passes a
   separate safety validator and user-approval boundary.
7. A new measurement window is compared with the original evidence by VERIFY.

## Replaceable boundaries

- `domain.TelemetrySource`: simulated data now; serial, Wi-Fi, or MQTT later.
- `domain.SessionRepository`: SQLite now; MongoDB Atlas or another store later.
- PROBE service: deterministic mock now; Gemini adapter later.
- Vision service: placeholder now; Gemini vision adapter later.
- Frontend API client: Go API with a browser fixture as a development fallback.

The browser fixture follows the same JSON contract as the Go API. It is not a
second product architecture; it exists to keep the hackathon demo runnable when
Go or hardware is unavailable.

## Trust boundaries

Device telemetry is untrusted until validated. Uploaded source and media are
untrusted until scanned. AI output is untrusted interpretation. PATCH is the only
component allowed to request an output signal, and its validator—not the model—has
the final decision.
