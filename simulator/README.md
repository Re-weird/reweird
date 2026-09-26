# Telemetry simulator

The Go API's `internal/simulator` package implements the same `TelemetrySource`
interface expected from a future ESP32 transport. It emits realistic telemetry
v2 raw windows for nine selectable healthy/fault cases. The scenario file
documents the built-in HC-SR04 probe roles and scenario IDs.

No frontend code depends on serial, Wi-Fi, or ESP32-specific behavior. Replace the
simulator with an authenticated device adapter that returns bounded
`TelemetryEnvelope` values and the analyzer, storage, evidence, and diagnostic
engine remain unchanged.
