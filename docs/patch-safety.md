# PATCH integration status — physical output disabled

The current ESP32-S3 breadboard has **no verified dedicated PATCH interface**.
P1–P6 and the OLED/USB/flash pins must not be repurposed. No output driver or
serial output command has been enabled. `/api/v1/patch` remains HTTP 423.
`REWEIRD_PATCH_ENABLE=true` cannot unlock it. Master enable starts OFF and this
release deliberately provides no physical-enable endpoint.

## Implemented

- Backend parameter allowlist and hard limits; server-generated action ID;
  device, boot, profile revision, probe-map and source bindings.
- Immutable parameter digest and verified-human identity requirement for
  one-use approval, expiring within 15 seconds. The proposal expires within
  60 seconds. Altered parameters require a new proposal and approval.
- Durable SQLite lifecycle/audit and restart invalidation (never resume or
  replay an interrupted action). Execution claims cannot be posted by clients.
- Driver contract for disable acknowledgement, fresh complete post-disable
  measurement window, source/identity checks and VERIFY callback.
- Mock-driver tests for the lifecycle, timeout/loss shutdown, exceptions,
  mismatches, replay and simulator isolation. This is **not hardware validation**.
- Locked capability advertisement in Telemetry v2; old firmware without that
  advertisement remains locked. Next-test UI displays PATCH LOCKED.

Physical proposals may be recorded using the authenticated project-scoped
`POST /api/v1/projects/:id/patch/proposals` route. They are marked LOCKED, not
approved or executed. Audit: `GET /api/v1/projects/:id/patch/actions`.
Status: `GET /api/v1/patch/status`. Existing owner/access checks apply.

## Required dedicated physical interface — not yet designed/qualified

Before a physical implementation, provide a reviewed schematic and measured
limits for ALL of the following:

1. A dedicated, explicitly assigned non-probe GPIO plus a separate hardware
   output-enable signal and labeled PATCH/GND connector. No GPIO is selected
   by this change. A floating measurement probe is not an output connector.
2. An external tri-state output/protection stage whose enable is held OFF by
   hardware at reset, boot, unpowered USB, firmware crash and brownout. A
   firmware `pinMode(INPUT)` alone is insufficient to qualify that behavior.
3. Independent hardware timeout/one-shot (maximum 250 ms) and a watchdog/lease
   (maximum 100 ms) that disables the stage when backend/USB heartbeat stops.
   Software cannot promise instantaneous detection of a disconnected process.
4. Rated series current limiting, overvoltage/back-power/ESD protection and
   documented source/sink current, leakage in high impedance, fault energy,
   settling time and behavior with the target powered while ReWeird is off.
5. A verified target-node allowlist: only isolated input nodes with no other
   active driver. No power rails, unknown outputs, mains, or ZMPT mains-side
   wiring. Isolation from the target's driver must be physically verified.

The software's **provisional model limits** are 3.3 V logic only, 1–250 ms,
and pulse trains at 1–100 Hz / 10–90% duty (at least one cycle). These are NOT
electrical ratings. No safe source/sink current or external-voltage tolerance
has been established, so **the presently permitted physical output is none**.
Do not build a circuit from these model limits without a qualified schematic.

## Remaining implementation gates

- Hardware schematic, dedicated pin and electrical qualification above.
- Boot-scoped capability/arming challenge, authenticated/bounded serial
  command decoder, independent firmware deadline/heartbeat enforcement,
  replay prevention and tested output-enable driver. Firmware currently
  rejects operation by having no active command/output path; limit assertions
  do not replace the missing physical driver/state machine.
- Connect the qualified driver to the backend controller; wire immediate
  approval/cancel UX with the exact target/mode/level/duration/digest.
- Connect automatic fresh real capture and the existing VERIFY evaluator.
  The current driver interface is tested with mocks only; it does not claim
  a real repair, automatically run PROBE, or change Layer 5.
- Expose simulated PATCH within Practice Simulator; currently the simulated
  lifecycle is exercised by tests, not a product UI feature.
- Physical unplug/reboot/timeout/collision tests using a scope and dummy load
  before connecting any target project. Then explicitly review master enable.

The attached broader plug-and-play integration checklist is not completed by
this PATCH-only change. Existing calibration/source-selection work is preserved.
