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
- A replaceable telemetry source interface and deterministic ESP32 simulator
- Project Profile, Live Diagnostics, Diagnosis, Verify, and Reports views
- Deterministic checks for stable power, expected activity, dropouts, and
  movement correlation
- Structured evidence that keeps measured values, rules, and interpretation
  separate
- A user-guided wiggle test and simulated repair flow
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
├── firmware/esp32/          # Future device protocol notes
├── packages/
│   ├── shared-types/        # Frontend contracts
│   └── component-catalog/   # Initial HC-SR04 specification
├── simulator/               # Demo scenario description
└── docs/                    # Architecture, diagnostics, and security notes
```

## Run the demo

### Prerequisites

- Node.js 20.9 or newer
- npm 10 or newer
- Go 1.23 or newer (optional for the browser-only demo)

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

The web app automatically detects the API at `http://localhost:8080`. The header
will show **API simulator**. Configure a different URL with
`NEXT_PUBLIC_API_URL`.

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
TelemetrySource (simulator today, ESP32 later)
        ↓
Normalized probe readings
        ↓
Deterministic diagnostic rules
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
| `GET` | `/api/v1/projects` | Project list |
| `GET` | `/api/v1/demo/session` | Current structured demo state |
| `POST` | `/api/v1/demo/wiggle` | Run the simulated wiggle test |
| `POST` | `/api/v1/demo/repair` | Simulate repair and re-measurement |
| `POST` | `/api/v1/demo/reset` | Reset the scenario |

State-changing demo transitions are written to SQLite as immutable diagnostic
snapshots.

## Safety model

- The LLM never receives or controls raw GPIO directly.
- The MVP performs input-only simulated tests; PATCH output is locked.
- Measurements, rule results, baselines, and AI interpretation have distinct
  fields and UI treatment.
- An inference cannot overwrite measured evidence.
- A future PATCH controller must validate pin, voltage, waveform, frequency, and
  duration before requesting explicit user approval.
- Git synchronization is disabled. A future adapter must scan for secrets,
  preserve actual authorship, and require user configuration and approval.
- Uploaded code and media will require type, size, and content validation before
  production use.

See [docs/security.md](docs/security.md) for the full trust-boundary checklist.

## Current limitations

- Telemetry is simulated; no serial or Wi-Fi ESP32 adapter is included yet.
- PROBE interpretation is a deterministic mock. Gemini is not connected.
- Vision and code parsing are interface placeholders only.
- SQLite stores session snapshots but project creation is currently a UI-only
  prototype.
- Authentication, device identity, multi-user access, and production upload
  scanning are not implemented.
- The chart uses summarized samples rather than a high-frequency time-series
  store.

## Future integrations

- **ESP32:** implement `domain.TelemetrySource` with an authenticated transport.
- **Gemini:** implement the PROBE and vision interfaces. Gemini must consume
  structured evidence and its response remains interpretation, never measured
  truth.
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
```
