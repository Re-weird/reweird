> Archived original hardware reference. Judge access and current defaults are in the [main README](../README.md). The original routes below require `REWEIRD_APP_MODE=hardware`; they are excluded from the public demo.

# ReWeird

![ReWeird logo](../apps/web/public/images/reweird-logo.png)

**Evidence-first diagnostics for physical electronics projects.**

ReWeird combines real measurements, project context, deterministic engineering
rules, guided follow-up tests, and repair verification. It is deliberately not a
chatbot that guesses at hardware failures.

> **Local/trusted demo only.** Do not expose the web app, Go API, PROBE, or
> optional understanding service directly to an untrusted or public network.
> The optional shared `REWEIRD_API_TOKEN` does not provide browser user
> authentication. See [the security model](security.md).

The included hackathon MVP demonstrates the complete loop:

> **Detect → Diagnose → Test → Verify**

The built-in project is an ESP32 connected to an HC-SR04 ultrasonic sensor. Its
ECHO line develops intermittent dropouts while the power rail and TRIG signal
remain stable. ReWeird isolates the anomaly, recommends a wiggle test, observes
that failures correlate with movement, simulates a repair, and verifies that the
signal returns to its healthy baseline.

## Judges: start here (no hardware needed)

Open [http://localhost:3000/demo](http://localhost:3000/demo) and pick one:

- **Project demo**: starts from the landing page, with our real ESP32 + HC-SR04 connected live.
- **Try it yourself**: a simulated circuit at `/try`. It needs no hardware, no
  Go API, no Python services and no sign-in:

```bash
npm install
npm run dev
```

In the simulation you:

1. **Make it weird.** Pick a fault, or let ReWeird secretly pick a **Mystery fault**.
2. **Investigate.** Read three simulated probes and choose a test. The right test
   changes the evidence. A wrong one says *That wasn’t it.* Hints are available.
3. **Fix.** Pick a simulated repair. A wrong one is allowed.
4. **VERIFY.** It shows **NOT WEIRD ANYMORE.** only when all four checks pass
   again. Otherwise it shows **STILL WEIRD.**
5. **Device Passport.** The whole run: time, tests, fixes and hints. You can
   download it as JSON.

A **SIMULATED DEMO** banner stays pinned throughout. Every simulated reading comes
from fixed tables in `apps/web/lib/judge-demo.ts`, and nothing is sent to a server.
The judge simulation lives only at `/try`; the project Workbench is kept for the
live hardware demo.

## What is included

- A Next.js + TypeScript web app: public landing page, dashboard, project list,
  settings, and per-project tabs (Workbench, Overview, Probe setup, Device
  passport, Simulator, Diagnosis, Next test, Verify result, History, Reports,
  Computer checks)
- A `/demo` page that lets judges choose the live project demo or the
  software-only `/try` simulation
- Optional Google sign-in; when configured, projects are scoped to the signed-in
  account, and anonymous demo mode keeps working without an account
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
- An interactive circuit map derived from each profile's components and
  connections; matching probe captures may overlay activity and per-probe checks
- User-confirmed Known Good captures linked to persisted measurement windows,
  and a Device Passport combining profile, map, source-labeled baselines,
  diagnostic history, verified outcomes, and concrete current status
- Deterministic checks for stable power, expected activity, dropouts, and
  movement correlation
- Structured evidence that keeps measured values, rules, and interpretation
  separate, with the Python PROBE service producing grounded hypotheses and a
  deterministic next-test recommendation through a fail-closed Go adapter
- A user-guided wiggle test and simulated repair flow
- A persisted, generic guided-test planner and before/during/after VERIFY
  workflow covering movement, rails, shared dropouts, activity, timing,
  trusted baselines, and re-measurement ([contract](guided-tests.md))
- Persisted diagnostic history, deterministic JSON/Markdown reports, and a
  print-friendly report view with provenance and secret redaction
- A separate computer-diagnostics simulator and optional read-only Windows
  collector; computer facts never become physical measurements
- Approval-gated Git synchronization for generated reports, disabled by default
- A Settings & status view that reports configuration without showing secrets
- A browser-side simulator fallback, so the demo still works if the Go API is
  not running
- A software-only judge simulation at `/try`: pick a fault or a mystery, run
  tests, try fixes, VERIFY, and get a downloadable Device Passport record
- An optional **Break our circuit** Workbench challenge that appears only for a
  matching live serial capture with confirmed probes

## Capability status

| Category | Current state |
| --- | --- |
| Working now | Project input and server-controlled confirmation, static code analysis, SQLite storage, generic signal analysis, user-confirmed source-labeled baselines, Device Passport, guided tests, VERIFY, history, deterministic reports |
| Simulated | Raw P1–P6 electrical scenarios, repair/verification demonstration, eight computer fault scenarios |
| Optional | Google sign-in with per-account project ownership; Gemini **Vision** for project photos and Gemini PROBE interpretation when configured; read-only Windows computer snapshot after opt-in; explicitly approved report Git commit/push after opt-in |
| Not yet hardware-validated | ESP32 firmware and USB serial ingestion compile but need electrical calibration and bench testing |
| Locked for safety | PATCH active electrical output, autonomous computer repair, unattended Git push |

Python PROBE diagnostic reasoning is integrated through `PROBE_SERVICE_URL`.
Without that service, or whenever it returns malformed/UNKNOWN output, the Go
API keeps its deterministic diagnosis unchanged. The UI never presents an AI
interpretation as measured fact.

## Repository map

```text
reweird/
├── apps/
│   ├── web/                 # Next.js diagnostic instrument UI
│   └── api/                 # Go/Fiber API and SQLite persistence
├── services/
│   ├── probe/               # AI reasoning interface boundary
│   ├── understand/          # Optional code + vision proposal service
│   ├── signal-analysis/     # Interface notes (analysis runs in apps/api)
│   └── vision/              # Interface notes (adapter runs in apps/api)
├── firmware/esp32/          # Passive probe firmware and PlatformIO project
├── packages/
│   ├── shared-types/        # Frontend contracts
│   └── component-catalog/   # Provenance-bearing component specifications
├── simulator/               # Demo scenario description
└── docs/                    # Architecture, diagnostics, and security notes
```

## Run locally

The [GitHub `main` branch](https://github.com/Re-weird/reweird) is the shared
source of truth. It includes the current logo, circuit map, Known Good capture,
and Device Passport. Feature branches are visible to the team but are not part
of the shared app until merged. The repository is public to read; contributing
or changing organization access requires the appropriate GitHub permissions.

`http://localhost:3000` is **only the app running on your own computer**. It
does not update when someone pushes to GitHub, and it is not a link teammates
can open on their own devices. To refresh a local copy, stop its old web/API
processes, run these commands in the checkout you intend to serve, then start
the stack below:

```bash
git fetch origin
git switch main
git pull --ff-only origin main
npm install
```

If `git switch` or `git pull --ff-only` refuses to proceed, preserve your local
changes and coordinate the merge; do not reset or force-push. If port 3000 is
already occupied, stop the old Next.js process before starting this checkout.
On Windows, `Get-NetTCPConnection -LocalPort 3000 -State Listen` identifies its
PID; inspect that process before stopping it. A hard refresh can clear cached
browser assets, but it cannot replace a server still running an old checkout.

There is **no public hosted ReWeird app** in this repository. Share the GitHub
link for code and documentation; each team member can run their own local copy.
Do not expose the current web/API stack through a public tunnel or host merely
to share `localhost`: browser user authentication and production upload safety
are not complete. See [security](security.md).

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
evidence panels are labelled **BROWSER SIMULATOR** and the built-in demo still
works. For judges, open [http://localhost:3000/demo](http://localhost:3000/demo). The
`npm run dev` script binds the web server to `127.0.0.1` by default so the
unfinished remote-access boundary is not exposed on a LAN.

### Full stack

Run the PROBE service in one terminal:

```bash
cd services/probe
uv run uvicorn app.api:app --host 127.0.0.1 --port 8091
```

Run the API in a second terminal:

```bash
cd apps/api
PROBE_SERVICE_URL=http://127.0.0.1:8091 go run ./cmd/server
```

Run the web app from the repository root in a third terminal:

```bash
npm install
npm run dev
```

The Next.js server proxies `/api/*` to `http://127.0.0.1:8080` by default, so the
browser uses one origin and evidence panels are labelled **API SIMULATOR** (or
**REAL SERIAL** with the ESP32 attached). Configure a
different server-side target with `API_INTERNAL_URL`; `NEXT_PUBLIC_API_URL`
remains available for deployments that intentionally expose a separate API
origin.

Security boundary: the standalone API binds to `127.0.0.1` by default.
Compose publishes ports 3000/8080/8091 on host loopback only. Do not publish
this stack to an untrusted network: Google sign-in is optional and the stack has
not been hardened for public hosting.
For direct API clients, setting a 32+ character `REWEIRD_API_TOKEN` requires
`Authorization: Bearer <token>` on `/api/v1` routes; this credential must stay
server-side. See [security model](security.md) before changing bind or
port settings.

Uploads default to `apps/api/data/uploads` when the API is started from that
directory. Override this with `UPLOAD_DIR`. To enable Gemini Vision, set the
key in the Go API process environment:

```powershell
$env:GEMINI_API_KEY = "your-key"
go run ./cmd/server
```

To enable Gemini PROBE reasoning, set `PROBE_AI_PROVIDER=gemini` and
`GEMINI_API_KEY` in the **Python PROBE process** as well. `GEMINI_MODEL` can be
configured per process; their defaults differ. Without a key, Python PROBE
uses its offline deterministic provider and image analysis reports
`VISION_SKIPPED`; code analysis, catalog enrichment, profile review,
confirmation, and probe planning continue normally.

### Optional Google sign-in

Sign-in is off unless these are set. In `apps/web` (for example `.env.local`):

```bash
NEXT_PUBLIC_GOOGLE_CLIENT_ID=...   # turns the sign-in UI on
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
AUTH_SECRET=...                    # NextAuth session secret
AUTH_TOKEN_SECRET=...              # shared with the Go API
```

Set the same `AUTH_TOKEN_SECRET` in the Go API process. The web app then sends a
short-lived signed token and the API scopes projects to the signed-in Google
account. Without it, every caller is treated as an anonymous demo user.

## Real Project Understanding flow

1. Select **New project** and enter name, description, controller, and logic
   voltage.
2. Upload a PNG/JPEG and upload or paste supported source code.
3. ReWeird persists the inputs, parses code deterministically, optionally asks
   Gemini Vision for suggestions, and generates a draft Project Profile.
4. Review provenance, edit components and connections, and resolve any code vs.
   vision conflict.
5. Confirm the profile. The backend checks conflicts and creates trusted probe
   assignments; this local action is not proof of a specific human identity.
6. Follow the generated GND/P1-P6 placement plan and confirm physical
   connections before opening Live Diagnostics.

The generic `/api/v1/profiles` write routes accept drafts only. Confirmation,
confirmation metadata, and probe assignments are server-controlled through the
project-specific workflow. A confirmed profile cannot be overwritten or
silently re-analyzed; a future revision workflow is required to change it.

The profile screen contains no universal HC-SR04 mapping. The built-in demo is a
normal seeded profile rendered by the same component. Its circuit map shows
intended connections and exact generated probe assignments; it only shows
measurement status for a matching, stored capture. It does not establish that
the physical wiring matches the profile.

The **Device Passport** lets a local operator select a stored healthy capture
and explicitly save it as Known Good. The backend checks the confirmed profile,
probe assignment, raw/derived capture identity, and required signal stability.
Serial captures become physical baselines only after the project's probe plan
was confirmed; simulator captures remain labeled simulated and are never used
for physical comparison. The confirmed profile is unchanged: baseline records
are separate, immutable SQLite records linked to measurement windows. A later
matching capture can show **Healthy**, **Deviation detected**, or **Needs
verification**; simulated comparisons use explicitly simulated labels. These
labels are comparisons, not physical certification or a numerical health score.
The ESP32 currently has no wall clock; the Passport uses backend ingestion time
when the device capture timestamp is unknown.
The API currently records `local_user_reported`, not an authenticated human
identity, so keep the stack local/trusted-network only.

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
valid frame has arrived. See [firmware/esp32/README.md](../firmware/esp32/README.md)
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
Go deterministic diagnosis → optional Python PROBE interpretation
        ↓
Go deterministic Test Planner (PROBE cannot bypass its safety checks)
        ↓
Guided test → re-measurement → verification
```

The frontend consumes only normalized JSON contracts. It does not know whether
telemetry came from a browser fixture, the Go simulator, or serial transport.
Device authentication is not implemented yet. That boundary lets real hardware
replace simulation without a UI rewrite.

## API

| Method | Endpoint | Purpose |
|---|---|---|
| `GET` | `/health` | Service health |
| `GET` | `/api/v1/session` | Current session from the selected telemetry source |
| `GET` | `/api/v1/ws/telemetry` (WebSocket) | Pushes the current session on connect, then again each time a new telemetry frame lands |
| `GET` | `/api/v1/telemetry/status` | Device/frame connection state |
| `GET` | `/api/v1/measurements` | Stored raw and derived measurement windows |
| `GET` | `/api/v1/simulator/scenarios` | List the nine deterministic raw-sample scenarios |
| `POST` | `/api/v1/simulator/scenario` | Select and analyze a simulator scenario |
| `GET` | `/api/v1/profiles` | Persistent Project Profiles |
| `GET` | `/api/v1/profiles/:id` | One Project Profile |
| `POST` | `/api/v1/profiles` | Validate and create a standalone draft profile only |
| `PUT` | `/api/v1/profiles/:id` | Update a standalone draft; cannot change project-backed or confirmed profiles |
| `GET` | `/api/v1/profiles/:id/passport` | Read the aggregated Device Passport and concrete status |
| `POST` | `/api/v1/profiles/:id/known-good` | Save one stored, user-confirmed healthy capture as a source-labeled baseline |
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
| `PUT` | `/api/v1/projects/:id/visibility` | Store private/public visibility (not enforced yet) |
| `GET` | `/api/v1/demo/session` | Current structured demo state |
| `GET` | `/api/v1/demo/probe-plan` | Probe plan for the built-in demo |
| `POST` | `/api/v1/demo/wiggle` | Run the simulated wiggle test |
| `POST` | `/api/v1/demo/repair` | Simulate repair and re-measurement |
| `POST` | `/api/v1/demo/reset` | Reset the scenario |
| `GET` `POST` | `/api/v1/tests`, `/tests/current`, `/tests/recommendation`, `/tests/:id` and `/tests/:id/{start,actions,capture,remeasure,cancel}` | Guided tests and VERIFY ([contract](guided-tests.md)) |
| `GET` | `/api/v1/history`, `/history/:id` | Diagnostic history |
| `GET` | `/api/v1/report`, `/reports/:id` | Deterministic JSON/Markdown reports |
| `GET` `POST` | `/api/v1/computer/{status,scenarios,simulate,collect}` | Computer diagnostics ([notes](computer-diagnostics.md)) |
| `GET` `POST` | `/api/v1/git/{status,preview/:id,commit/:id}` | Approval-gated report Git sync ([notes](git-sync.md)) |
| `GET` | `/api/v1/status` | Settings and configuration status (no secrets) |
| `POST` | `/api/v1/patch` | Always returns `423 PATCH_LOCKED` |

Go API demo transitions are written to SQLite as immutable diagnostic
snapshots.

Open **Fault simulator** to select any scenario and inspect the complete software
path from raw samples through validation, normalization, profile matching,
measurement storage, structured evidence, diagnosis, guided change, and VERIFY.
The simulator does not inject a diagnosis label into the engine; each result is
derived from the raw electrical values and the confirmed profile.
The judge simulation at `/try` is separate: it runs entirely in the browser and
does not call the API. On the Workbench, the optional **Break our circuit**
challenge only appears for a matching serial capture and confirmed probe setup;
it never controls electrical output.

## Safety model

- The LLM never receives or controls raw GPIO directly.
- P1–P6 remain input-only sensing. The PATCH lifecycle is implemented, but the
  current board's physical output stays locked without qualified dedicated
  hardware. See [PATCH provisioning and safety](patch-safety.md).
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
- Project images are limited to 5 MiB each; one current image is retained, with
  at most two files and 11 MiB image-plus-code storage during replacement.
  Obsolete images are removed after the new reference is persisted. The optional
  understanding service also bounds its request body, source-file list, total
  source text, and decoded image size.
- Gemini can only suggest `VISION_AI` facts. Conflicts are preserved, and only a
  user can confirm a Project Profile.
- An inference cannot overwrite measured evidence.
- PATCH validates the dedicated target/pin, voltage, mode, duration, device boot,
  profile/mapping and action digest before explicit user approval. Master enable
  defaults OFF. Firmware enforces bounded output and lease shutdown independently;
  fresh REAL_SERIAL capture and VERIFY follow acknowledged disable. Practice
  PATCH is an explicitly simulated lifecycle rehearsal, never physical evidence.
- Git synchronization is disabled by default. The report adapter scans for
  secrets and requires local configuration and explicit per-commit approval;
  remote push needs an additional opt-in and approval.
- Uploaded code and media have current type, size, and content checks; a
  production deployment still needs malware scanning and authentication.

See [docs/security.md](security.md) for the full trust-boundary checklist.

## Current limitations

- Firmware and serial ingestion are implemented and compile, but were not flashed
  or electrically bench-tested because no physical board was available.
- Passport status describes the **last stored capture**, not continuously
  certified device health. ESP32 reboot identity and clock synchronization
  still need bench validation; a conflicting reused telemetry identity is
  rejected rather than overwriting an existing measurement.
- USB serial is the only real transport; Wi-Fi, WebSocket, and MQTT adapters are
  not implemented.
- PROBE diagnostic interpretation runs in `services/probe` when
  `PROBE_SERVICE_URL` is configured. The Go adapter submits only structured
  evidence and the deterministic diagnosis; it falls back to that diagnosis
  verbatim on any request, transport, UNKNOWN, or validation failure.
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
- Google sign-in is optional, and API ownership scoping only applies when
  `AUTH_TOKEN_SECRET` is configured. Project visibility is stored but not
  enforced. Device identity, team roles, and production upload scanning are not
  implemented, and a shared API token is not a browser login; do not expose the
  stack publicly.
- The chart uses summarized samples rather than a high-frequency time-series
  store.

## Future integrations

- **ESP32:** bench-test and calibrate the passive front end, then add device
  authentication and an optional network transport.
- **Gemini:** Vision remains a Go adapter. PROBE's optional Gemini provider is
  in the Python service behind `PROBE_AI_PROVIDER=gemini`; its output is
  grounding-validated before main can use the mapped diagnosis. Remaining work
  is prompt tuning against real hardware evidence and a live-key integration
  test, not the interpretation-only boundary itself.
- **MongoDB Atlas:** add a repository adapter for Project Profiles, sessions,
  baselines, and reports without changing domain logic.
- **Time-series storage:** retain raw high-rate samples outside the relational
  session store, then feed summaries into diagnostics.
- **Git:** add authenticated, auditable unattended workflows only after a
  separate approval and security design. Current report commits are manual.

## Development checks

```bash
npm run typecheck
npm run build
npm run test:experience --workspace @reweird/web
npm audit

cd apps/api
go test ./...
go vet ./...
govulncheck ./...

cd ../../services/probe
uv run pytest

cd ../understand
uv run pytest

cd ../../firmware/esp32
pio run
```
