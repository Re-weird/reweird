# Telemetry simulator

The Go API's `internal/simulator` package implements the same `TelemetrySource`
interface expected from a future ESP32 transport. The scenario file documents the
built-in HC-SR04 demo states and probe roles.

No frontend code depends on serial, Wi-Fi, or ESP32-specific behavior. Replace the
simulator with an authenticated device adapter that returns normalized `ProbeReading`
values and the diagnostic engine can remain unchanged.
