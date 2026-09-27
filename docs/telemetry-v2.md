# Telemetry v2 contract

Layer 3 hardware integrations implement `TelemetrySource`; they do not call the
diagnostic engine directly. The simulator and implemented ESP32 USB-serial
transport both produce the same bounded `TelemetryEnvelope`.

```json
{
  "schema_version": 2,
  "device_id": "reweird-esp32-001",
  "profile_id": "ultrasonic-demo",
  "captured_at_ms": 0,
  "uptime_ms": 120000,
  "window_ms": 1000,
  "sequence": 120,
  "samples": [
    { "probe": "P1", "mode": "analog", "analog_mv": [2501, 2507, 2499] },
    {
      "probe": "P2",
      "mode": "pulse",
      "state": 0,
      "edge_count": 80000,
      "rising_edges": 40000,
      "falling_edges": 40000,
      "periods_us": [25.0, 24.9, 25.1],
      "high_pulse_widths_us": [10.0, 9.9, 10.1],
      "max_gap_us": 26,
      "activity_counts": [4000, 3998, 4002]
    }
  ]
}
```

USB serial sends exactly one compact JSON object per newline. The backend rejects
unknown fields, unsupported versions, invalid device/profile identifiers,
duplicate probes, unsafe ADC values, non-finite numbers, inconsistent edge
counts, oversized arrays, unconfirmed profiles, cross-profile frames, and probe
modes that differ from the confirmed profile.

## Window semantics (clarified, fields unchanged)

Every timing value describes one capture window only:

- `max_gap_us` is the longest interval inside the window with no edge, counting
  from the window start to the first edge and from the last edge to the window
  end. With no edges it equals the window length. It can never exceed
  `window_ms × 1000`.
- `periods_us` and `high_pulse_widths_us` are reported only when both edges are
  inside the window. A pulse straddling a window boundary contributes its edges
  to the counts but no width or period.
- `sequence` increases per device across resets: devices without a wall clock
  (`captured_at_ms` = 0) keep a boot counter in the upper 32 bits.

The backend does not clamp values that violate these rules. It marks the probe
`capture_unreliable`, lists the reason in `capture_issues`, keeps the raw value,
and refuses to use that probe as evidence for a circuit diagnosis or a Known
Good baseline. Frames from older firmware whose gaps span several windows are
reported this way.

Raw values are device-side observations. The backend applies the confirmed
`input_scale`, derives engineering units and signal facts, compares them with
specifications and trusted baselines, then stores the original envelope and the
derived analysis together. The ESP32 never sends diagnosis labels.

PATCH is outside this contract. A telemetry source is input-only and cannot ask
the backend or target project to drive a pin.
