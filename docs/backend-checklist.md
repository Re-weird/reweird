# Backend status

This is a current implementation summary, not a claim of production readiness.
For setup and capability details see the [README](../README.md); for deployment
boundaries see the [security model](security.md).

## Implemented in software

- Go/Fiber API, SQLite projects/profiles, measurement windows, diagnostic
  history, reports, and browser telemetry WebSocket.
- Validated telemetry v2 ingestion with replaceable simulator and USB-serial
  sources; generic signal analysis and deterministic diagnostic rules.
- Project image and source intake, deterministic GPIO extraction, optional
  Gemini Vision, catalog enrichment, conflict-preserving draft profiles, and
  server-controlled profile confirmation and probe-plan generation.
- Python PROBE service for optional grounded interpretation, with a
  deterministic Go fallback when the service or model is unavailable.
- Persisted guided-test planning and before/during/after VERIFY, including
  simulator scenarios. PATCH-required actions remain locked.
- Separate simulated computer diagnostics and an opt-in, read-only Windows
  collector; approval-gated report Git sync is off by default.

## Not validated or not implemented

- The ESP32 firmware and serial path need bench calibration, voltage-safety
  checks, and reconnect/timeout testing with physical hardware.
- PATCH cannot energize a pin: the API returns `423 PATCH_LOCKED` and firmware
  configures its PATCH GPIO as input. No active-output safety case exists.
- The browser has no accounts or per-user authorization. Shared bearer tokens
  and reporter metadata do not prove human identity. Do not deploy the stack
  directly to an untrusted network.
- Device authentication, production malware scanning, remote TLS deployment,
  and a live-key Gemini integration test remain future work.
- The built-in simulator is intentionally tied to its demo profile. A newly
  created profile requires matching real telemetry before its own physical
  diagnosis and VERIFY path can be validated end to end.
