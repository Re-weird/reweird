# ReWire Project Context

## 1. Project Summary

**ReWire** is an **AI-assisted hardware and computer diagnostic system**.

It helps a user understand why an electronics or computer project is not working by combining:

- project photo/video
- project source code
- component specifications
- real probe measurements
- software/computer telemetry
- deterministic engineering checks
- Gemini reasoning
- guided testing
- safe PATCH testing
- VERIFY re-measurement
- reports and Git history

> **Core idea:** ReWire learns what a project should do, measures what it is doing, finds the mismatch, guides the next test, and checks whether the problem is fixed.

---

## 2. Problem We Are Solving

When an electronics project fails, the problem may be:

- power
- wiring
- sensor/component failure
- wrong GPIO
- firmware
- configuration
- computer software
- network/service problems
- a mismatch between software intent and physical hardware

Today, users usually jump between tools such as a multimeter, serial monitor, logs, IDE, documentation, and AI chat.

ReWire brings those signals into one guided workflow.

---

## 3. Main Product Goals

ReWire should:

1. Understand the project.
2. Understand what the project is supposed to do.
3. Measure what is really happening.
4. Compare expected vs actual behavior.
5. Explain the likely issue.
6. Ask for the next useful test.
7. Optionally run a safe PATCH test.
8. Re-measure the project.
9. VERIFY whether the issue is fixed.
10. Create a report and optional Git history.

---

## 4. Product Modes

### Hardware Mode
Uses ReWire probes to diagnose electronics.

Examples:
- missing power
- missing digital signal
- bad PWM
- no sensor response
- unstable signal
- intermittent connection

### Computer Mode
Works without the ESP32 hardware.

Checks can include:
- CPU
- memory
- disk
- network
- processes
- services
- selected logs
- system settings
- selected security checks

### Combined Mode
Uses hardware + software data together.

Example:

```text
Software says PWM = 70%
Physical probe sees PWM = 0%
=> mismatch
```

---

## 5. Core Trust Rule

> **The LLM reasons about evidence. It does not create evidence.**

Gemini must never:

- invent a measurement
- change a recorded measurement
- directly control the ESP32
- directly toggle GPIO
- bypass the Go safety layer
- directly write to Git

---

## 6. Final User Flow

1. Create/open a project.
2. Upload project photo/video.
3. Upload project code.
4. Add an optional short description.
5. ReWire identifies visible parts.
6. ReWire reads the project code.
7. ReWire checks the Component Catalog.
8. ReWire builds a proposed Project Profile.
9. User selects **Confirm Project Profile**.
10. User connects ReWire probes.
11. ESP32 starts measuring.
12. Signal Analysis turns raw readings into useful facts.
13. Diagnostic Engine checks:
   - Rules
   - Specs
   - Baseline if available/trusted
   - Software-vs-Hardware evidence
14. PROBE reads structured evidence.
15. PROBE explains the likely issue.
16. Test Planner chooses the next useful test.
17. If an active test needs approval, user selects **Approve Test Action**.
18. User performs the test or PATCH runs after safety checks.
19. ReWire measures again.
20. VERIFY checks the result.
21. ReWire creates a report.
22. User decides whether files should be saved to Git.
23. If approved: **Approved Auto Commit -> Auto Push**.
24. If not approved: **Local only**.

---

## 7. Important User Actions

### Confirm Project Profile
Used after ReWire builds the proposed Project Profile.

The user reviews:
- components
- connections
- GPIO roles
- expected behavior

### Approve Test Action
Used later when an active test needs approval.

Example:
- PATCH test

These two actions are intentionally separate.

---

## 8. Project Profile

The Project Profile is ReWire's project-specific map.

Example:

```text
PROJECT: Parking Sensor

Controller:
ESP32

Components:
- HC-SR04
- Buzzer
- LED

Connections:
GPIO5  -> HC-SR04 TRIG
GPIO18 -> HC-SR04 ECHO
GPIO21 -> Buzzer
GPIO22 -> LED

Expected behavior:
- GPIO5 sends trigger pulses
- GPIO18 receives echo pulses
- distance < 20 cm -> buzzer ON
- distance > 20 cm -> buzzer OFF
```

The Project Profile should store:

- project ID/name
- controller
- components
- connections
- GPIO roles
- expected signal types
- expected behavior
- main rules
- safety limits
- source of each fact
- confidence for AI guesses
- optional trusted baseline reference

---

## 9. What ReWire Learns

For V1, ReWire mainly learns by storing project context.

It does not need to retrain a model for every project.

It stores:

- confirmed Project Profile
- components
- GPIO roles
- expected behavior
- safe limits
- test history
- diagnosis history
- VERIFY history
- optional trusted healthy baseline

---

## 10. Baseline Rules

- Baseline is optional.
- Baseline is only trusted when the user confirms the project is healthy.
- Never save the first readings from an already-broken project as normal.
- A new project can still be diagnosed with Rules + Specs + Software evidence.
- If there is not enough information, return **UNKNOWN** or ask for another test.

---

# 11. Layer 1 - User / Input Layer

## Tech Stack
- TypeScript
- React
- Next.js

## Purpose
Collect project information and user approvals.

## Responsibilities
- upload photo/video
- upload project code
- optional short description
- guide probe connection
- show proposed Project Profile
- let user correct findings
- Confirm Project Profile
- Approve Test Action
- show final report
- show Git history

## Rule
The browser talks to the Go backend only.

The browser should not directly call Gemini or the ESP32.

---

# 12. Layer 2 - Project Understanding Layer

## Tech Stack
- Python
- FastAPI
- Pydantic
- Google Gemini API
- Google GenAI SDK
- Tree-sitter

## Vision
Gemini Vision suggests visible parts such as:
- ESP32
- HC-SR04
- servo
- LED
- breadboard
- resistor
- buzzer

Vision results are suggestions, not final truth.

## Code Analysis
Use deterministic parsing first where possible.

Example:

```cpp
#define TRIG 5
#define ECHO 18
#define BUZZER 21
```

Tree-sitter or normal parsing should extract exact facts.

Gemini helps understand higher-level logic.

## Component Catalog
The Catalog stores:

- component name/type
- supply voltage
- pin roles
- logic levels
- signal types
- safe ranges
- timing behavior
- known relationships
- common fault patterns
- supported tests
- PATCH support
- safety limits

## Project Builder
Combines:

- vision
- code facts
- Catalog
- user description
- user corrections

Output:
- proposed Project Profile

---

# 13. Layer 3 - Hardware / Firmware Layer

## Tech Stack
- ESP32
- C++
- Arduino framework or ESP-IDF
- CD74HC4067 multiplexer
- USB Serial

## ReWire Hardware
- P1
- P2
- P3
- P4
- P5
- P6
- GND
- PATCH

## Firmware Responsibilities
- control MUX channels
- sample probes
- measure supported analog/digital values
- timestamp readings
- send telemetry
- report device state
- receive checked commands from Go
- execute supported PATCH signals
- reject invalid commands

## Example Telemetry

```json
{
  "type": "telemetry",
  "v": 1,
  "device_id": "rewire-01",
  "probe": "P1",
  "voltage": 3.28,
  "state": "HIGH",
  "timestamp_ms": 123456
}
```

## Example PATCH Command

```json
{
  "type": "command",
  "v": 1,
  "command": "patch_test",
  "output": "PATCH",
  "mode": "HIGH",
  "duration_ms": 500
}
```

## Hardware Safety
PATCH is a test signal, not a universal repair tool.

Do not connect V1 to:
- mains
- unknown high voltage
- high-current motor power
- unknown power rails

---

# 14. Layer 4 - Go Backend / Data Layer

## Tech Stack
- Go
- Fiber
- WebSockets
- sqlc
- SQLite

## Purpose
Go is the main control center.

## Go Owns
- API
- app state
- projects
- sessions
- Project Profiles
- Catalog access
- serial connection
- measurement storage
- Diagnostic Engine
- Evidence Builder
- PATCH safety
- VERIFY
- reports
- audit history
- Git flow
- frontend API
- Python AI service calls

## Suggested Go Folders

```text
backend/
  cmd/
  api/
  device/
  projects/
  catalog/
  measurements/
  diagnostics/
  evidence/
  safety/
  verify/
  reports/
  gitlog/
  audit/
  db/
```

## Important Rule
Go is the only software layer allowed to approve physical control.

Python may suggest a test.

Go decides whether it is allowed.

---

# 15. Layer 5 - Signal Analysis Layer

## Tech Stack
- Go for simple signal math
- Python
- NumPy
- SciPy for harder signal analysis

## Signal Facts
Possible outputs:

- voltage
- min/max voltage
- HIGH/LOW
- pulse count
- frequency
- PWM duty cycle
- jitter
- dropout count
- stability
- timing
- cross-probe relationships

Example:

```text
Probe: P3
High: 3.30 V
Low: 0.04 V
Frequency: 9.98 Hz
Dropouts: 17
Stability: 72%
```

Gemini should receive structured facts, not large raw sample streams.

---

# 16. Diagnostic Engine

## Tech Stack
Mainly Go.

## Four Evidence Sources

### 1. Rules
Simple engineering checks.

### 2. Specs
Compare with Component Catalog facts.

### 3. Baseline
Compare with trusted known-good behavior.

Only if available.

### 4. Software vs Hardware
Compare software intent with physical signals.

Example:

```text
Software:
PWM requested = 70%

Hardware:
PWM = 0%

=> mismatch
```

---

# 17. Structured Evidence

PROBE receives structured evidence.

Example:

```json
{
  "finding_id": "ev-104",
  "project_id": "parking-sensor",
  "signal": "GPIO18",
  "expected": "echo pulse after trigger",
  "actual": "no pulse detected",
  "supporting_evidence": [
    "measurement:P2",
    "measurement:P3",
    "catalog:HC-SR04",
    "project:GPIO18"
  ]
}
```

---

# 18. Layer 6 - Intelligence / PROBE Layer

## Tech Stack
- Python
- FastAPI
- Pydantic
- Google Gemini API

## PROBE Can
- explain likely causes
- explain findings in simple words
- choose the next test
- return confidence
- return evidence IDs
- return UNKNOWN
- propose a PATCH test

## PROBE Cannot
- create measurements
- change measurements
- control GPIO
- control ESP32
- bypass Go safety
- directly save Git commits

## Example PROBE Output

```json
{
  "finding": "possible_echo_path_fault",
  "confidence": 0.87,
  "evidence_ids": ["P2", "P3", "HC-SR04"],
  "next_test": "check_echo_connection"
}
```

---

# 19. Layer 7 - Test Planning Layer

## Tech Stack
- Python/Gemini for choosing the test
- Go for test state

Possible tests:

- check power
- compare probes
- reseat a connection
- wiggle test
- isolate a signal
- check software intent
- run PATCH test

Test state:

- proposed
- waiting
- approved
- running
- complete
- unclear

---

# 20. Layer 8 - Safety / Control Layer

## Tech Stack
- Go
- ESP32 firmware safety limits

Before PATCH runs, Go checks:

- requested output
- pin
- signal type
- voltage/mode
- duration
- project safety limits
- component safety limits
- user approval if required

If unsupported:

```text
REJECT
```

If supported:

```text
APPROVE
  ↓
send checked command to ESP32
```

---

# 21. Layer 9 - VERIFY / Report Layer

## Tech Stack
- Go
- Python only for extra signal analysis when needed

VERIFY flow:

```text
Before
  ↓
Repair / Test / PATCH
  ↓
Re-measure
  ↓
Compare
  ↓
VERIFY
```

Possible results:

- fixed
- still failing
- unclear

---

# 22. Layer 10 - Computer Diagnostics Layer

## Tech Stack
- Go local agent
- OS tools
- Python/Gemini when useful

Possible data:

- CPU
- memory
- disk
- network
- processes
- services
- selected logs
- configuration
- selected security checks

Combined Mode sends software evidence into the same Diagnostic Engine.

---

# 23. Layer 11 - Git / Software Log Layer

## Tech Stack
- Go
- Git CLI

## Final Git Flow

```text
Diagnostic/report generated
        ↓
Security scan
        ↓
Determine source
        ↓
Human author / ReWire Bot
        ↓
Approved for Git?
      /             YES          NO
     ↓            ↓
Approved       Local only
Auto Commit
     ↓
Auto Push
     ↓
History
```

## Important Rule
Git does not magically know who physically made a change.

ReWire explicitly uses:
- Human author
- ReWire Bot

## Example Commit Metadata

```text
Source: ReWire Diagnostic Engine
Session: RW-014
Action: VERIFY completed
Files:
  reports/RW-014.json
  diagnostics/history.json
Result: P3 intermittent behavior resolved
```

---

# 24. Layer 12 - Frontend / Output Layer

## Tech Stack
- TypeScript
- React
- Next.js
- Tailwind CSS
- shadcn/ui
- Recharts

## Main Screens
- project setup
- photo/code upload
- Confirm Project Profile
- probe setup
- live measurements
- expected vs actual
- evidence
- diagnosis
- next test
- Approve Test Action
- PATCH status
- VERIFY result
- report
- Git history

---

# 25. Full End-to-End Data Flow

```text
Photo / Video
      +
Project Code
      +
Short Description
      +
Probe Connections
      ↓
Project Understanding
      ↓
Vision + Code Analysis + Catalog
      ↓
Proposed Project Profile
      ↓
Confirm Project Profile
      ↓
ESP32 Measures
      ↓
Signal Analysis
      ↓
Diagnostic Engine
      ↓
Rules + Specs + Baseline if trusted + Software-vs-Hardware
      ↓
Structured Evidence
      ↓
PROBE (Gemini)
      ↓
Test Planner
      ↓
User Test
or
Approve Test Action
      ↓
Go Safety Check
      ↓
PATCH if supported
      ↓
Re-measure
      ↓
VERIFY
      ↓
Generate Report
      ↓
Security Scan
      ↓
Determine Source
      ↓
Approved for Git?
   /        YES        NO
  ↓          ↓
Commit     Local only
  ↓
Auto Push
  ↓
History
```

---

# 26. Team of 4

## Person 1 - Hardware + Firmware ONLY

Owns:
- ESP32
- MUX
- probes
- firmware
- sampling
- serial messages
- device status
- PATCH output
- firmware safety limits
- hardware test setup

---

## Person 2 - Entire Go Backend

Owns:
- Fiber API
- WebSockets
- serial Device Manager
- SQLite
- sqlc
- projects
- sessions
- Catalog service
- Project Profiles
- measurement storage
- Diagnostic Engine
- Evidence Builder
- safety checks
- VERIFY
- reports
- audit logs
- Git automation

---

## Person 3 - AI / Python

Owns:
- Python AI service
- FastAPI
- Pydantic
- Gemini API
- vision
- Tree-sitter/code analysis
- proposed Project Profile
- PROBE
- Test Planner
- NumPy/SciPy helpers
- AI tests/evals

---

## Person 4 - Frontend + Integration

Owns:
- TypeScript
- Next.js
- project setup
- uploads
- Confirm Project Profile UI
- live charts
- diagnosis UI
- test UI
- Approve Test Action UI
- PATCH status
- VERIFY UI
- report UI
- Git history UI
- full demo integration

---

# 27. Shared Contracts To Define First

The team should agree on:

- exact demo project
- exact fault scenarios
- probe naming
- PATCH safety limits
- serial JSON v1
- ProjectProfile JSON
- EvidencePackage JSON
- ProbeFinding JSON
- TestRequest JSON
- PatchRequest JSON
- REST endpoints
- WebSocket event names

---

# 28. Suggested Repository Layout

```text
rewire/
  firmware/          # Person 1

  backend/           # Person 2
    cmd/
    api/
    device/
    projects/
    catalog/
    measurements/
    diagnostics/
    evidence/
    safety/
    verify/
    reports/
    gitlog/
    audit/
    db/

  ai/                # Person 3
    api/
    vision/
    code_analysis/
    project_builder/
    probe/
    test_planner/
    signals/
    schemas/
    evals/

  frontend/          # Person 4
    app/
    components/
    features/
    lib/

  catalog/
  contracts/
  fixtures/
  reports/
  docs/
  docker-compose.yml
```

---

# 29. Recommended Build Order

## Step 1 - Lock contracts
Define:
- demo circuit
- serial messages
- shared JSON
- safety limits
- API names

## Step 2 - Make data move

```text
ESP32 -> Go -> SQLite -> WebSocket -> Frontend
```

## Step 3 - Build project understanding

```text
Photo + Code + Catalog
        ↓
Project Profile
        ↓
Confirm Project Profile
```

## Step 4 - Build diagnostics

```text
Measurements
   ↓
Signal Facts
   ↓
Rules + Specs + Baseline + Software Checks
   ↓
Structured Evidence
```

## Step 5 - Add PROBE

```text
Structured Evidence
        ↓
Gemini
        ↓
Finding + Next Test
```

## Step 6 - Add test loop

```text
Next Test
   ↓
User Test / PATCH
   ↓
New Measurements
```

## Step 7 - Add VERIFY

```text
Before vs After
      ↓
Fixed / Still Failing / Unclear
```

## Step 8 - Add Report + Git

```text
Report
  ↓
Security Scan
  ↓
Determine Source
  ↓
Approved for Git?
  ↓
Commit or Local Only
```

## Step 9 - Demo hardening

Test:
- ESP32 disconnect
- bad data
- Python service failure
- Gemini failure
- invalid PATCH request
- user correction
- repeated full demo

---

# 30. V1 Demo Target

Use one known demo project.

Example:
- ESP32
- HC-SR04
- LED
- buzzer or servo

Demo flow:

1. Upload photo.
2. Upload code.
3. ReWire finds parts.
4. ReWire builds Project Profile.
5. User confirms it.
6. Live probe values appear.
7. Introduce one controlled fault.
8. Diagnostic Engine finds evidence.
9. PROBE explains the issue.
10. PROBE asks for a test.
11. Run a user test or safe PATCH.
12. ReWire measures again.
13. VERIFY shows the result.
14. Generate report.
15. Approve for Git.
16. Show ReWire Bot history.

---

# 31. V1 Non-Goals

Do not try to:

- support every electronics project
- repair every problem automatically
- connect to dangerous voltages
- let Gemini control hardware
- train a custom ML model before the main system works
- call first readings normal
- force a diagnosis when evidence is weak

---

# 32. Testing Plan

## Firmware
Test:
- MUX channel selection
- probe reading
- serial messages
- bad command rejection
- PATCH limits

## Go
Test:
- serial parser
- rules
- specs
- safety checks
- VERIFY
- database
- API
- Git flow

## Python
Test:
- Pydantic schemas
- parser fixtures
- Gemini prompts
- evidence ID checking
- UNKNOWN cases

## Frontend
Test:
- project setup
- Confirm Project Profile
- live values
- diagnosis
- Approve Test Action
- VERIFY
- report

## Full System
Test:
- missing signal
- intermittent connection
- wrong GPIO
- power issue
- software says output ON but hardware signal is missing

---

# 33. Definition of Done

V1 is done when:

- photo + code can build a useful Project Profile
- user can Confirm Project Profile
- ESP32 probe data reaches Go
- live data appears in the UI
- one controlled fault creates structured evidence
- PROBE explains the evidence
- PROBE suggests a useful test
- PATCH, if used, goes through Go safety checks
- Approve Test Action works when needed
- VERIFY re-measures and checks the result
- a report is generated
- Git uses Human author / ReWire Bot
- Git only commits approved files
- the full demo can run repeatedly

---

# 34. Final Tech Stack

| Part | Tech | Owner |
|---|---|---|
| Hardware / Firmware | ESP32, C++, Arduino framework or ESP-IDF, CD74HC4067, USB Serial | Person 1 |
| Main Backend | Go, Fiber, WebSockets | Person 2 |
| Database | SQLite, sqlc | Person 2 |
| Diagnostics | Go | Person 2 |
| Safety / PATCH | Go + ESP32 firmware limits | Person 2 + Person 1 |
| AI Service | Python, FastAPI, Pydantic | Person 3 |
| LLM / Vision | Google Gemini API, Google GenAI SDK | Person 3 |
| Code Analysis | Tree-sitter + normal parsing + Gemini | Person 3 |
| Signal Analysis | Go for simple math, NumPy/SciPy for harder work | Person 2 + Person 3 |
| Frontend | TypeScript, React, Next.js, Tailwind CSS, shadcn/ui, Recharts | Person 4 |
| Git / Logs | Go + Git CLI | Person 2 |
| Local Dev | Docker + Docker Compose | Software team |
| Testing | Go tests, pytest, frontend tests, hardware fixtures | Everyone |

---

# 35. Final One-Line Architecture

```text
ESP32/C++
    ↓
Go/Fiber
    ↓
Python/FastAPI + Gemini
    ↓
SQLite/sqlc
    ↓
TypeScript/Next.js
    ↓
Approve Test Action
    ↓
VERIFY
    ↓
Report
    ↓
Approved Git
```

---

# 36. Final Build Rule

> **ESP32 measures. Go owns the system and safety. Python/Gemini understands and explains. TypeScript shows the workflow. The user confirms the Project Profile and approves active test actions when needed.**
