# Security model

## Current controls

- PATCH output is disabled in the MVP.
- The UI labels the AI-style interpretation as mocked and keeps it separate from
  measured values.
- The simulator and future hardware implement the same narrow telemetry contract.
- SQLite receives normalized diagnostic snapshots, not arbitrary device commands.
- CORS allows only local development origins.
- Git synchronization is off by default.

## Required before real hardware

- Authenticate every device and rotate device credentials.
- Reject unknown probe identifiers and malformed or out-of-range telemetry.
- Apply rate limits, replay protection, monotonic timestamps, and payload limits.
- Define per-project safe voltage, waveform, frequency, duration, and pin policies.
- Require a user-visible approval step before any PATCH action that can energize a
  target circuit.
- Include a hardware current limit and physical output-disable control.
- Audit whether every action came from a user, deterministic system rule, AI
  recommendation, or hardware event.

## Required before uploads and integrations

- Validate MIME type, extension, size, and archive expansion limits.
- Scan source and reports for secrets before persistence or Git operations.
- Store API keys server-side only; never expose them to the browser or firmware.
- Sandbox code parsing and media processing.
- Escape report output and guard against prompt injection in source comments,
  component text, and uploaded documents.
- Preserve real Git authorship. Generated content may be attributed to a ReWeird
  bot only when the bot actually makes the commit.
