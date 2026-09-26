# ReWeird ESP32 passive probe firmware

This firmware runs on the **ReWeird diagnostic device**, not the target project.
It samples six passive probe inputs and emits newline-delimited telemetry v2 JSON
over USB serial. PATCH is deliberately configured as an input and has no output
code path.

## Default probe map

| Probe | ESP32 pin | Mode | Captured data |
|---|---:|---|---|
| P1 | GPIO 34 | Analog | 32 bounded millivolt samples per window |
| P2 | GPIO 25 | Pulse | state, edges, periods, high widths, activity buckets |
| P3 | GPIO 26 | Pulse | state, edges, periods, high widths, activity buckets |
| P4 | GPIO 27 | Digital | state, transitions, activity buckets |
| P5 | GPIO 32 | Digital | state, transitions, activity buckets |
| P6 | GPIO 33 | Digital | state, transitions, activity buckets |
| PATCH | GPIO 4 | **Locked input** | Nothing; active output is not implemented |

Change pins or modes in `include/probe_config.h`. The backend's confirmed Project
Profile must use the same assignments.

## Electrical safety assumptions

The ESP32 is **not a multimeter** and cannot safely measure arbitrary voltages.

- Never allow any ESP32 input pin to exceed **3.3 V** or go below ground.
- Never connect a 5 V signal or HC-SR04 ECHO directly to an ESP32 input. Use a
  verified divider or level shifter.
- The demo P1 profile uses `input_scale: 2`, which assumes a verified 2:1 divider.
  The firmware still reports the voltage actually seen by the ADC pin; the backend
  applies the confirmed scale.
- Connect ReWeird ground to the target's safe low-voltage ground reference.
- Do not use this prototype on mains, high-energy circuits, inductive power stages,
  automotive systems, or unknown voltage sources.
- GPIO interrupt measurements are approximate. Validate bandwidth and loading on
  the assembled diagnostic PCB before trusting high-frequency results.
- PATCH should remain physically disconnected during this phase.

## Build and flash

Install [PlatformIO](https://platformio.org/install), connect an ESP32 DevKit, and
run from this directory:

```bash
pio run
pio run --target upload
pio device monitor --baud 115200
```

On Windows, the legacy Xtensa compiler may fail with `CreateProcess: No such file
or directory` when the repository and PlatformIO package paths are very long.
Build from a shorter checkout path or temporarily map the project and PlatformIO
core directories to drive letters.

The monitor should show one compact JSON object per line every second. Each frame
uses `packages/shared-types/telemetry.schema.json` and includes a schema version,
device ID, configured `profile_id`, sequence, measurement window, and P1-P6
samples. Update `REWEIRD_PROFILE_ID` with the probe configuration so the backend
can reject accidental cross-project telemetry.

Find the serial port when needed:

```bash
pio device list
```

## Connect the backend

Confirm that the Project Profile describes the physical divider, probe roles, and
expected signals, then start the API in serial mode. PowerShell example:

```powershell
$env:TELEMETRY_MODE = "serial"
$env:SERIAL_PORT = "COM5"
$env:SERIAL_BAUD = "115200"
$env:PROJECT_PROFILE_ID = "ultrasonic-demo"
go run ./cmd/server
```

The API rejects unknown fields, invalid schema versions, duplicate or unknown
probes, oversized arrays, invalid states, non-finite values, mismatched modes, and
unconfirmed profiles. Check connection state at:

```text
GET http://localhost:8080/api/v1/telemetry/status
```

No real board was available during implementation. The firmware was compiled for
`esp32dev`, but flashing, divider calibration, probe loading, and bench validation
must be performed on the actual ReWeird hardware.
