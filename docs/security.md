# Security model

> **Not safe for public deployment.** ReWeird currently assumes a local or
> explicitly trusted hackathon network. Keep all published ports on loopback;
> the shared API token is not an end-to-end browser login or per-user identity.

## Current controls

- PATCH output is disabled in the MVP.
- The standalone Go API binds to loopback by default. A non-loopback bind
  requires either a 32+ character `REWEIRD_API_TOKEN` (Bearer header on all
  `/api/v1` routes, including WebSocket upgrade) or explicit
  `API_TRUSTED_NETWORK=true`. The latter is for isolated container networks,
  never a substitute for authentication on a public interface.
- Docker Compose publishes web, API, and PROBE ports on host loopback only.
  Its internal bridge is treated as trusted; it is not a remote-deployment
  security boundary. Do not expose these ports publicly.
- PROBE refuses PATCH-result recording unless a 32+ character
  `PROBE_REPORTER_TOKEN` and server-configured `PROBE_REPORTER_ID` are present.
  The record contains actor ID, server timestamp, and `HUMAN_REPORTED`
  provenance. Neither approval nor execution is attested by the hardware;
  `execution_verified` remains false. Offline demo records are explicitly
  marked `OFFLINE_SIMULATION`.
- ESP32 firmware configures PATCH as an input and contains no output action path.
- The UI keeps PROBE interpretation separate from measured values. With the
  offline provider or browser fixture it is simulated; optional Gemini output
  remains an interpretation, never a measurement.
- The simulator and firmware implement the same versioned telemetry contract.
- Serial JSON rejects unknown fields, unsupported schema versions, duplicate or
  unknown probes, invalid states, non-finite/out-of-range values, oversized
  arrays, mismatched modes, and unconfirmed profiles.
- The API body limit is 6 MiB. Images are limited to 5 MiB and accepted only when
  content sniffing identifies PNG or JPEG. Code is limited to 512 KiB, restricted
  to supported text extensions, validated as UTF-8/non-binary, and never
  compiled, imported, evaluated, or executed.
- Generic `/api/v1/profiles` writes accept drafts only, cannot change
  project-backed or confirmed profiles, and cannot set confirmation metadata
  or generated probe configurations. Only the project confirmation endpoint
  can confirm a profile after conflict checks and probe-plan generation.
- Each project keeps one current image. A second image may exist only during
  replacement; after the new reference is saved, obsolete project images are
  removed. At most two image files and 11 MiB of image-plus-code storage are
  allowed during replacement; the current image is still limited to 5 MiB and
  code to 512 KiB. Existing diagnostic records are not deleted by image cleanup.
- The optional understanding service limits `/understand` requests to 8 MiB,
  eight source files, 512 KiB per source file, 1 MiB total source text, and
  a 5 MiB decoded image. Oversized requests return 413 or validation 422.
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

## Required before remote deployment

- Add user accounts/sessions and role-based authorization for the browser UI.
  A shared API bearer token is service access control, not per-user identity;
  the current browser proxy and WebSocket client do not carry that token.
- Put the services behind TLS and an authenticated reverse proxy. Keep the
  Compose host mappings on loopback until that path is implemented and tested.
- Give each action reporter a distinct credential and rotate/revoke it as
  needed. A shared `PROBE_REPORTER_ID` identifies the configured credential,
  not a cryptographically proven human or physical device action.

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
