# PATCH integration status — physical output disabled

The current ESP32-S3 breadboard has **no verified dedicated PATCH interface**.
P1–P6 and the OLED/USB/flash pins must not be repurposed. The provisionable driver
and command protocol are implemented, but physical execution on this board stays
LOCKED. `/api/v1/patch` remains HTTP 423: arbitrary GPIO commands are never allowed.
`REWEIRD_PATCH_ENABLE=true` cannot unlock anything. Master enable starts OFF on
every backend restart and is bound to device boot, profile and mapping.

The same production API/web build discovers a newly qualified device without
an application-code rewrite: LOCKED → READY → ARMED → ACTIVE → DISABLED.
Readiness alone never enables output or grants approval. Device firmware must
carry its reviewed hardware qualification record and detect its physical
interlock; a client capability claim or environment variable is insufficient.
On the current board the UI reads **PATCH SOFTWARE READY** and
**PHYSICAL PATCH LOCKED — protected output hardware not detected**.

## Implemented

- Backend parameter allowlist and hard limits; server-generated action ID;
  device, boot, profile revision, probe-map and source bindings.
- Immutable parameter digest and verified-human identity requirement for
  one-use approval, expiring within 15 seconds. The proposal expires within
  60 seconds. Altered parameters require a new proposal and approval.
- Durable SQLite lifecycle/audit and restart invalidation (never resume or
  replay an interrupted action). Execution claims cannot be posted by clients.
- Serial driver with boot/challenge handshake, exact approved parameter digest,
  bounded DIGITAL/PULSE only, heartbeat, disable acknowledgement, and automatic
  post-disable capture selected by device uptime (never by a guessed host clock).
- Existing backend VERIFY evaluates stored Before/After windows using the frozen
  confirmed profile plus applicable Known Good. Firmware performs no diagnosis.
- Firmware timer enforcement independent of the measurement loop: 250 ms maximum
  duration, 100 ms maximum lease, physical interlock, one-use arming, replay cache,
  strict bounded command decoder, digest check and disabled default. Safety
  shutdown/error is distinguished from successful completion.
- Authenticated, owner-scoped approval/cancel and default-off master control.
  Next Test displays exact target, pin, level, voltage, duration and capture IDs;
  parameters cannot be edited after proposal. Changes require a fresh proposal.
- Mock-wire HTTP lifecycle, controller, serial demultiplexing, frontend and native
  firmware state-machine tests. These are **not physical qualification**.
- Practice Simulator has an isolated, explicitly SIMULATED lifecycle rehearsal.
  It sends no hardware requests and creates no measurement/Known Good records.
  Its VERIFY is explicitly inconclusive without measured scenario evidence.

API workflow (all mutations require authenticated project ownership):

- `GET /api/v1/patch/status`: LOCKED / READY / ARMED / ACTIVE / DISABLED.
- `POST /api/v1/projects/:id/patch/master`: `{enabled,confirm}`; enabling requires
  a matching provisioned hardware capability. An environment flag is insufficient.
- `POST /api/v1/projects/:id/patch/prepare`: `{level,duration_ms}`; target/pins/profile
  come from the verified firmware provisioning, not arbitrary client GPIO fields.
- `POST /api/v1/projects/:id/patch/proposals`: structured planner proposal; never
  executes. Current unprovisioned devices retain a LOCKED audit record.
- `POST /api/v1/projects/:id/patch/actions/:action/approve`: `{digest,confirm:true}`;
  immediate one-use execution, disable, fresh capture and VERIFY.
- `POST /api/v1/projects/:id/patch/actions/:action/cancel`: disable and revoke.
- `GET /api/v1/projects/:id/patch/actions`: persisted lifecycle/results/capture IDs.

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

The implemented physical driver supports **3.3 V bounded HIGH/LOW only, 1–250 ms**.
Pulse trains are not supported by the physical driver. These software limits
are NOT electrical ratings. No safe source/sink current or external-voltage
tolerance has been established, so **presently permitted physical output is none**.
Do not build a circuit from these model limits without a qualified schematic.

## Provisioning and remaining physical gates

- Hardware schematic, dedicated pin and electrical qualification above.
- After qualification, review `firmware/esp32/include/patch_provision.h`: assign
  three distinct safe GPIOs (data, active-high OE with external pull-down, physical
  interlock with external pull-down), qualification ID, exact profile/revision,
  mapping hash and one isolated target node. That node must be measured by a
  confirmed probe connection so VERIFY has meaningful evidence. Provisioning is
  a reviewed firmware build record, never writable by API or environment flag.
- The serial command channel assumes exclusive access by the trusted local API.
  It is not a cryptographically authenticated remote-device protocol. Do not
  expose the serial channel to untrusted software/network clients.
- On a qualified build, closing the physical interlock permits capability
  advertisement. A separate human master-enable and exact-action approval are
  still required. Missing/changed boot/profile/mapping fails closed.
- Physical unplug/reboot/timeout/collision tests using a scope and dummy load
  before connecting any target project. Then explicitly review master enable.

Telemetry v2 measurements remain unchanged. Commands/replies use distinct JSON
`patch_command`/`patch_status` types, never display text; replies are demultiplexed
before measurement parsing. Production emits control replies only to commands.
The shipped device remains unprovisioned and does not need reflashing tonight.

Native firmware tests: compile `host-test/patch_machine_test.cpp` with a C++17
compiler and `-Iinclude`, then run the executable. No board or electrical output
is used. Go tests include the mocked-wire end-to-end approval/capture workflow.
The actual command parser also has a Windows native test (BCrypt-backed SHA-256):
from `firmware/esp32`, compile `host-test/patch_runtime_test.cpp` with C++17,
`-DREWEIRD_PATCH_HOST_TEST=1 -DCONFIG_IDF_TARGET_ESP32S3=1`, include paths
`host-test/patch-mock`, `include`, `.pio/libdeps/esp32s3/ArduinoJson/src`, and
link `-lbcrypt`. Its fake pins/provisioning are host-only test fixtures;
`ESP_PLATFORM` production builds cannot select them.
