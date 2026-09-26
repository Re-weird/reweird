# ReWeird

**Evidence-first diagnostics for physical electronics projects.**

ReWeird combines real measurements, project context, deterministic engineering
rules, guided follow-up tests, and repair verification. It is deliberately not a
chatbot that guesses at hardware failures.

The included hackathon MVP demonstrates the complete loop:

> **Detect → Diagnose → Test → Verify**

The built-in project is an ESP32 connected to an HC-SR04 ultrasonic sensor. Its
ECHO line develops intermittent dropouts while the power rail and TRIG signal
remain stable. ReWeird isolates the anomaly, recommends a wiggle test, observes
that failures correlate with movement, simulates a repair, and verifies that the
signal returns to its healthy baseline.

## What is included

- A polished Next.js + TypeScript diagnostic dashboard
- A Go + Fiber API with SQLite diagnostic snapshots
- A replaceable telemetry source interface with simulator and USB-serial ESP32 implementations
- Compilable ESP32 firmware for passive P1-P6 ADC, digital, edge, pulse-period,
  pulse-width, and activity-window measurements
- A versioned, strictly validated telemetry v2 contract with explicit Project Profile matching
- Nine realistic raw-telemetry scenarios: healthy, dead signal, low voltage,
  unstable power, missing pulses, intermittent connection, simultaneous
  dropouts, timing drift, and software-controlled change
- Durable SQLite measurement windows preserving both original samples and derived facts
- Generic signal analysis for voltage statistics, transitions, frequency, duty
  cycle, jitter, missing activity, dropouts, simultaneous failures, and trusted
  baseline deviation
- Persistent, user-confirmable Project Profiles in SQLite
- Persisted Project records with validated image upload and upload-or-paste code
  input
- Deterministic Arduino/ESP32 and MicroPython GPIO analysis that never executes
  uploaded source
- Optional Gemini Vision component/relationship suggestions with forced
  `VISION_AI` provenance and a graceful no-key path
- A provenance-aware Project Understanding merge engine with visible conflict
  resolution and user corrections
- A nine-entry component catalog covering HC-SR04, SG90-style servo, LED, push
  button, generic digital I/O, PWM, I2C, and UART
- Generic probe placement generated only after user confirmation, followed by a
  persisted "probes connected" gate
- Project Profile, Live Diagnostics, Diagnosis, Verify, and Reports views
- Deterministic checks for stable power, expected activity, dropouts, and
  movement correlation
- Structured evidence that keeps measured values, rules, and interpretation
  separate
- A user-guided wiggle test and simulated repair flow
- A persisted, generic guided-test planner and before/during/after VERIFY
  workflow covering movement, rails, shared dropouts, activity, timing,
  trusted baselines, and re-measurement ([contract](docs/guided-tests.md))
- A browser-side simulator fallback, so the demo still works if the Go API is
  not running

## Repository map

```text
reweird/
├── apps/
│   ├── web/                 # Next.js diagnostic instrument UI
│   └── api/                 # Go/Fiber API and SQLite persistence
├── services/
│   ├── probe/               # AI reasoning interface boundary
│   ├── signal-analysis/     # Raw telemetry → structured facts boundary
│   └── vision/              # Image analysis interface boundary
├── firmware/esp32/          # Passive probe firmware and PlatformIO project
├── packages/
│   ├── shared-types/        # Frontend contracts
│   └── component-catalog/   # Provenance-bearing component specifications
├── simulator/               # Demo scenario description
└── docs/                    # Architecture, diagnostics, and security notes
```

## Run the demo

### Prerequisites

- Node.js 20.9 or newer
- npm 10 or newer
- Go 1.25 or newer (optional for the browser-only demo)
- PlatformIO 6 or newer (only for building/flashing ESP32 firmware)

### Fastest path: frontend with built-in simulator

```bash
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). If the API is unavailable,
the header shows **Browser simulator** and every demo action still works.

### Full stack

Run the API in one terminal:

```bash
cd apps/api
go mod tidy
go run ./cmd/server
```

Run the web app from the repository root in another terminal:

```bash
npm install
npm run dev
```

The Next.js server proxies `/api/*` to `http://127.0.0.1:8080` by default, so the
browser uses one origin and the header shows **API simulator**. Configure a
different server-side target with `API_INTERNAL_URL`; `NEXT_PUBLIC_API_URL`
remains available for deployments that intentionally expose a separate API
origin.

Uploads default to `apps/api/data/uploads` when the API is started from that
directory. Override this with `UPLOAD_DIR`. To enable Gemini Vision and the
Gemini PROBE adapter (they share the same key and model), keep the key
server-side:

```powershell
$env:GEMINI_API_KEY = "your-key"
$env:GEMINI_MODEL = "gemini-3.5-flash-lite" # optional
go run ./cmd/server
```

Without a key, PROBE falls back to the deterministic mock diagnosis and
image analysis reports `VISION_SKIPPED`; both continue to work.

Without a key, image analysis reports `VISION_SKIPPED`; code analysis, catalog
enrichment, profile review/confirmation, and probe planning continue normally.

## Real Project Understanding flow

1. Select **New project** and enter name, description, controller, and logic
   voltage.
2. Upload a PNG/JPEG and upload or paste supported source code.
3. ReWeird persists the inputs, parses code deterministically, optionally asks
   Gemini Vision for suggestions, and generates a draft Project Profile.
4. Review provenance, edit components and connections, and resolve any code vs.
   vision conflict.
5. Confirm the profile. Only this user action makes the model authoritative.
6. Follow the generated GND/P1-P6 placement plan and confirm physical
   connections before opening Live Diagnostics.

The profile screen contains no universal HC-SR04 mapping. The built-in demo is a
normal seeded profile rendered by the same component.

### Real ESP32 over USB serial

The firmware is a PlatformIO project under `firmware/esp32`. Read its safety
notes before attaching a target circuit.

```bash
cd firmware/esp32
pio run
pio run --target upload
pio device list
```

Start the backend with the board's port. PowerShell example:

```powershell
cd apps/api
$env:TELEMETRY_MODE = "serial"
$env:SERIAL_PORT = "COM5"
$env:SERIAL_BAUD = "115200"
$env:PROJECT_PROFILE_ID = "ultrasonic-demo"
go run ./cmd/server
```

The active Project Profile must be user-confirmed and its probe modes must match
the firmware configuration. `GET /api/v1/telemetry/status` reports whether a
valid frame has arrived. See [firmware/esp32/README.md](firmware/esp32/README.md)
for the wiring assumptions and exact connection procedure.

The ultrasonic simulator always remains bound to `ultrasonic-demo`. Confirming
probes for another project opens Live Diagnostics in a waiting state until a
matching serial telemetry source is available; demo samples are never relabeled
as measurements from the uploaded project.

### Docker Compose

```bash
docker compose up --build
```

Then open [http://localhost:3000](http://localhost:3000).

## Demo script

1. Start on **Overview** and note that P1 power and P2 TRIG are healthy while P3
   ECHO has 12 dropouts per minute.
2. Open **Diagnosis**. Compare expected, observed, and baseline evidence. The
   system describes plausible causes but does not claim a loose wire.
3. Select **Start wiggle test**. The simulator raises the P3 dropout rate to 27
   while P1 and P2 remain stable.
4. Review the stronger movement-correlation evidence and select **Simulate
   repair**.
5. The app opens **Verify**. P3 now has 0 dropouts per minute and matches the
   stored healthy baseline.
6. Open **Reports** to download the structured session snapshot.
7. Select **Reset demo** on the Verify screen to repeat the flow.

## How the system works

```text
TelemetrySource (simulator or ESP32 USB serial)
        ↓
Validated telemetry v2 window + confirmed profile match
        ↓
Durable raw measurement storage
        ↓
Signal analysis (voltage, edges, frequency, duty, pulse width, jitter, gaps, dropouts)
        ↓
Project Profile + specifications + trusted baseline
        ↓
Generic deterministic diagnostic rules
        ↓
Structured evidence
        ↓
PROBE interpretation (mocked in this MVP)
        ↓
Guided test → re-measurement → verification
```

The frontend consumes only normalized JSON contracts. It does not know whether
telemetry came from a browser fixture, the Go simulator, serial transport, or a
real authenticated ESP32. That boundary lets real hardware replace simulation
without a UI rewrite.

## API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/health` | Service health |
| `GET` | `/api/v1/session` | Current session from the selected telemetry source |
| `GET` | `/api/v1/telemetry/status` | Device/frame connection state |
| `GET` | `/api/v1/measurements` | Stored raw and derived measurement windows |
| `GET` | `/api/v1/simulator/scenarios` | List the nine deterministic raw-sample scenarios |
| `POST` | `/api/v1/simulator/scenario` | Select and analyze a simulator scenario |
| `GET` | `/api/v1/profiles` | Persistent Project Profiles |
| `GET` | `/api/v1/profiles/:id` | One Project Profile |
| `POST` | `/api/v1/profiles` | Validate and create a Project Profile |
| `PUT` | `/api/v1/profiles/:id` | Validate and update a Project Profile |
| `POST` | `/api/v1/projects` | Create a persisted project shell |
| `GET` | `/api/v1/projects` | List persisted projects |
| `GET` | `/api/v1/projects/:id` | Read a project and its input/analysis metadata |
| `POST` | `/api/v1/projects/:id/media` | Validate and store a PNG/JPEG upload |
| `POST` | `/api/v1/projects/:id/code` | Validate and store uploaded or pasted source |
| `POST` | `/api/v1/projects/:id/analyze` | Generate deterministic/vision analysis and draft profile |
| `GET` | `/api/v1/projects/:id/profile` | Read the generated profile |
| `PUT` | `/api/v1/projects/:id/profile` | Persist user corrections/conflict resolutions |
| `POST` | `/api/v1/projects/:id/profile/confirm` | User-confirm the profile and generate a probe plan |
| `GET` | `/api/v1/projects/:id/probe-plan` | Read generic probe placement instructions |
| `POST` | `/api/v1/projects/:id/probe-plan/confirm` | Persist physical probe connection confirmation |
| `GET` | `/api/v1/demo/session` | Current structured demo state |
| `POST` | `/api/v1/demo/wiggle` | Run the simulated wiggle test |
| `POST` | `/api/v1/demo/repair` | Simulate repair and re-measurement |
| `POST` | `/api/v1/demo/reset` | Reset the scenario |
| `POST` | `/api/v1/patch` | Always returns `423 PATCH_LOCKED` |

State-changing demo transitions are written to SQLite as immutable diagnostic
snapshots.

Open **Fault simulator** to select any scenario and inspect the complete software
path from raw samples through validation, normalization, profile matching,
measurement storage, structured evidence, diagnosis, guided change, and VERIFY.
The simulator does not inject a diagnosis label into the engine; each result is
derived from the raw electrical values and the confirmed profile.

## Safety model

- The LLM never receives or controls raw GPIO directly.
- Firmware and backend perform input-only sensing; PATCH output is locked.
- ESP32 frames are schema-versioned and bounded. Telemetry v2 requires a
  `profile_id`, and the backend rejects frames that do not match the active
  confirmed profile. It also rejects unknown
  fields, duplicate probes, invalid states, unsafe ADC values, mismatched probe
  modes, oversized arrays, and unconfirmed profiles.
- ESP32 pins must never see more than 3.3 V. Higher low-voltage signals require a
  verified divider or level shifter recorded in the Project Profile.
- Measurements, rule results, baselines, and AI interpretation have distinct
  fields and UI treatment.
- Uploaded code is bounded UTF-8 text data. It is never compiled, executed,
  imported, or evaluated. Secret-like filenames and binary content are rejected.
- Images are content-sniffed as PNG/JPEG, stored under generated names inside a
  dedicated root, and bounded before Gemini input.
- Gemini can only suggest `VISION_AI` facts. Conflicts are preserved, and only a
  user can confirm a Project Profile.
- An inference cannot overwrite measured evidence.
- A future PATCH controller must validate pin, voltage, waveform, frequency, and
  duration before requesting explicit user approval.
- Git synchronization is disabled. A future adapter must scan for secrets,
  preserve actual authorship, and require user configuration and approval.
- Uploaded code and media will require type, size, and content validation before
  production use.

See [docs/security.md](docs/security.md) for the full trust-boundary checklist.

## Current limitations

- Firmware and serial ingestion are implemented and compile, but were not flashed
  or electrically bench-tested because no physical board was available.
- USB serial is the only real transport; Wi-Fi, WebSocket, and MQTT adapters are
  not implemented.
- PROBE diagnostic interpretation has an optional Gemini adapter
  (`internal/probe`) that rewords the deterministic rule engine's finding
  in plain English; it never invents new evidence and falls back to the
  deterministic wording verbatim on any request, transport, or validation
  failure, and whenever `GEMINI_API_KEY` is unset.
- The deterministic code analyzer covers common Arduino/ESP32 C/C++ and basic
  MicroPython patterns. It is intentionally not a full compiler and reports
  unsupported or unresolved constructs instead of guessing.
- A Tree-sitter adapter is not enabled in the portable `CGO_ENABLED=0` API build;
  the code-analyzer interface allows one when a deployment provides CGO and a C
  compiler.
- One project image and one source payload are retained as the current inputs;
  multi-file projects, video, GitHub import, OCR-only documents, and archives are
  not supported yet.
- A real Gemini request requires the operator's API key and network access. The
  adapter and structured-response parser are tested with a local fake endpoint,
  not a live paid key in this repository.
- Authentication, device identity, multi-user access, and production upload
  scanning are not implemented.
- The chart uses summarized samples rather than a high-frequency time-series
  store.

## Future integrations

- **ESP32:** bench-test and calibrate the passive front end, then add device
  authentication and an optional network transport.
- **Gemini:** the vision and PROBE interfaces are both implemented behind
  the same key-gated swap pattern. Remaining work is prompt tuning against
  real hardware evidence and a live-key integration test, not the
  interpretation-only boundary itself.
- **MongoDB Atlas:** add a repository adapter for Project Profiles, sessions,
  baselines, and reports without changing domain logic.
- **Time-series storage:** retain raw high-rate samples outside the relational
  session store, then feed summaries into diagnostics.
- **Git:** add an opt-in, secret-scanned report adapter that records the real
  change source.

## Development checks

```bash
npm run typecheck
npm run build

cd apps/api
go test ./...

cd ../../firmware/esp32
pio run
```
