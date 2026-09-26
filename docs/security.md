# Security model

## Current controls

- PATCH output is disabled in the MVP.
- ESP32 firmware configures PATCH as an input and contains no output action path.
- The UI labels the AI-style interpretation as mocked and keeps it separate from
  measured values.
- The simulator and firmware implement the same versioned telemetry contract.
- Serial JSON rejects unknown fields, unsupported schema versions, duplicate or
  unknown probes, invalid states, non-finite/out-of-range values, oversized
  arrays, mismatched modes, and unconfirmed profiles.
- The API body limit is 6 MiB. Images are limited to 5 MiB and accepted only when
  content sniffing identifies PNG or JPEG. Code is limited to 512 KiB, restricted
  to supported text extensions, validated as UTF-8/non-binary, and never
  compiled, imported, evaluated, or executed.
- Original filenames are reduced to safe basenames. Media is stored under a
  generated content-hash name inside the configured upload root, with path
  containment checks and exclusive file creation.
- Obvious credential uploads such as `.env`, `.pem`, `.key`, `.p12`, and `.pfx`
  are rejected. Full source text is not written to request logs.
- Gemini credentials remain server-side. Image input is bounded, and every
  returned fact is forced to `VISION_AI` provenance before merging.
- SQLite receives normalized diagnostic snapshots, not arbitrary device commands.
- CORS allows only local development origins.
- Git synchronization is off by default.
- Report exports redact recognized credentials and values of configured secret
  environment variables. Git preview refuses commits if redaction was needed
  or a generated artifact still matches a credential pattern. The scanner is
  not a substitute for human review.
- Git writes only two new `.reweird/` report artifacts, with path/symlink
  checks, local-request approval, and no force push. It uses the configured Git
  author instead of impersonating a teammate.
- Real computer collection is opt-in, loopback-only, time-bounded, and read
  only. It does not collect command lines, file contents, or arbitrary logs.
- Raw measurement storage fails closed at `MAX_MEASUREMENT_WINDOWS` (default
  50,000, allowed 100–1,000,000); it never silently deletes evidence. Image,
  code, telemetry-frame, report, and computer-snapshot sizes are also bounded.
  Operators may lower ceilings with `MAX_IMAGE_BYTES` (≤5 MiB),
  `MAX_CODE_BYTES` (≤512 KiB), `MAX_TELEMETRY_ANALOG_SAMPLES` (≤256 per
  probe), `MAX_TELEMETRY_PULSE_SAMPLES` (≤512 per probe), `MAX_REPORT_BYTES`
  (≤1 MiB), and `MAX_COMPUTER_SNAPSHOT_BYTES` (≤1 MiB). Invalid values fall
  back to the audited defaults; they cannot raise the hard caps.

## Required before real hardware

- Authenticate every device and rotate device credentials.
- Add cryptographic device identity; USB serial currently identifies but does not
  authenticate the board.
- Apply rate limits, replay protection, monotonic timestamps, and payload limits.
- Define per-project safe voltage, waveform, frequency, duration, and pin policies.
- Require a user-visible approval step before any PATCH action that can energize a
  target circuit.
- Include a hardware current limit and physical output-disable control.
- Audit whether every action came from a user, deterministic system rule, AI
  recommendation, or hardware event.

## Required before production uploads and integrations

- Add malware scanning and content-disarm policy if additional media formats are
  accepted. Archives remain unsupported, so there is no archive expansion path.
- Scan source and reports for secrets beyond the current filename blocklist
  before persistence or Git operations.
- Store API keys server-side only; never expose them to the browser or firmware.
- Sandbox code parsing and media processing.
- Escape report output and guard against prompt injection in source comments,
  component text, and uploaded documents.
- Preserve real Git authorship. Generated content may be attributed to a ReWeird
  bot only when the bot actually makes the commit.
