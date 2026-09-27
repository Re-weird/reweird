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

### ESP32-S3 probe board (`env:esp32s3`)

The S3 has no GPIO 25/26/34 and uses 26-37 for flash/PSRAM, so it gets its own
map, matching the S3 board wiring in `firmware.ino`:

| Probe | ESP32-S3 pin | Mode | Capture |
|---|---:|---|---|
| P1 | GPIO 8 (ADC1) | Analog | 32 mV samples per window |
| P2 | GPIO 3 | Pulse | MCPWM hardware capture (12.5 ns timestamps) |
| P3 | GPIO 16 | Pulse | MCPWM hardware capture |
| P4 | GPIO 21 | Digital | GPIO interrupt |
| P5 | GPIO 9 (ADC1) | Analog | 32 mV samples per window (ZMPT OUT) |
| P6 | GPIO 48 | Digital | GPIO interrupt |
| PATCH | none | Not configured | — |

Telemetry goes out on the S3's native USB port (`ARDUINO_USB_CDC_ON_BOOT=1`).

Hardware capture latches edge polarity and time, so a 10 µs HC-SR04 trigger is
not lost to interrupt latency. It cannot detect a signal that never crosses
the input threshold: the S3 needs about 2.5 V (0.75 × 3.3 V) for a guaranteed
HIGH. A 3.3 V trigger through a 2:1 divider arrives at about 1.65 V and will be
missed. Only divide signals that can exceed 3.3 V (for example a 5 V ECHO).

P5 samples ZMPT OUT 32 times per second. That shows its DC level and a coarse
spread, not a true AC amplitude or RMS: a 50/60 Hz waveform is undersampled.

The final S3 hardware uses **two independent I2C controllers**, not a TCA:
- OLED A: TwoWire(0), SDA GPIO17, SCL GPIO18, SSD1306 128x64 at 0x3C.
- OLED B: TwoWire(1), SDA GPIO4, SCL GPIO5, SSD1306 128x64 at 0x3C.

Disconnect/remove the failed TCA and its old display routing. Keep the buses'
SDA/SCL separate, and share ground. Use 3.3 V supply for modules rated for 3.3 V;
verify the actual module rating. Never pull ESP32 I2C lines up to 5 V.
Each SDA and SCL needs a suitable pull-up to 3.3 V. Module onboard resistors may
already provide these: do not blindly parallel another set. If absent, 4.7k
per line is a starting value, subject to rise-time/wiring validation at 400 kHz.
[Espressif I2C guidance](https://docs.espressif.com/projects/esp-idf/en/v5.2/esp32s3/api-reference/peripherals/i2c.html).

Both buses begin explicitly at 400 kHz before display initialization. Installed
Adafruit SSD1306 2.5.17 has begin(switchvcc, address, reset, periphBegin);
both displays use begin(SSD1306_SWITCHCAPVCC, 0x3C, false, false), preserving
custom buses/pins. No production address scan, channel selection or routing
depends on the old multiplexer. Earlier standalone TCA diagnostics are historical.

Both briefly show ReWeird branding, then A shows P1 POWER / P2 TRIG / P3 ECHO,
and B shows P4 SERVO / P5 ZMPT / P6 SPARE plus REAL SERIAL / PATCH LOCKED.
Both consume the SAME completed Telemetry v2 measurement window. Voltage is
at the ESP32 input pin (no display-side divider scaling); ZMPT is not RMS.
OLED failures stay in the display task on core 0, never blocking telemetry.
The surviving display reports the other as OFF; unavailable displays are retried.

Normal esp32s3 firmware remains JSON-only. The serial-monitor-only
esp32s3-displaydiag build reports bus initialization and per-display ACK/init,
then repeats a compact status every 10 seconds after a delayed 3-second check:
`DISPLAY_STATUS OLED_A=BUS0:OK OLED_B=BUS1:OK`.
A missing/noninitializing OLED is `OFFLINE`, not a fabricated success.
Keep the API stopped for this diagnostic build; text is not valid API input.
Measurements still stream unchanged. Restore esp32s3 before restarting the API.
ACK/init success does not prove visible pixels. Neither build uses the TCA.

The profile ID sent in every frame must match the confirmed Project Profile the
API runs. Set it when building instead of editing code (PowerShell):

```powershell
$env:REWEIRD_PROFILE_ID = "your-project-id"
pio run -e esp32s3
pio run -e esp32s3 -t upload --upload-port COM5
pio device monitor -p COM5 -b 115200
```

The ESP32-S3 build now refuses to compile without `REWEIRD_PROFILE_ID`; it
must be the confirmed physical project's profile ID, not `ultrasonic-demo`.

### Window semantics

Each frame covers one window. `max_gap_us`, `periods_us`, and
`high_pulse_widths_us` never span windows (see `docs/telemetry-v2.md`). The
`sequence` keeps increasing across resets: a boot counter stored in flash (one
write per boot) forms its upper 32 bits.

`host-test/run.sh` compiles `src/main.cpp` for a PC with mocked Arduino/ESP-IDF
APIs and simulated bench signals, to check the window accounting. It is not a
hardware test.

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
