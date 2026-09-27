# Hosted physical demo: USB bridge and judge link

The website remains on Vercel and the API remains on Railway. Only the read-only
USB bridge runs on the laptop attached to the ESP32. No firmware change or public
laptop server is required. The bridge does not carry camera video or PATCH commands.

## Owner setup

1. Sign in to the hosted site and open the physical project, not Practice Simulator.
2. Confirm its profile and probe placement. They must describe the real board,
   including P5 analog. Local profiles/baselines are not automatically copied to
   the hosted database; confirm the hosted configuration before pairing.
3. On Workbench open **Connect USB hardware to this hosted project / Share with judges**.
4. Enter the exact device ID and firmware profile ID from real telemetry. Confirm
   the mapping, then **Pair USB bridge**. This is an explicit alias between the
   existing firmware profile ID and the cloud project: no source-code reflash.
5. Stop the local API and serial monitors so COM5 is free. In `apps/api`, run the
   PowerShell commands shown by the UI. Paste the private bridge token only into
   the secure prompt. Requires Go; alternatively build `go build ./cmd/usb-bridge`.
6. Keep the laptop awake, connected to USB and the internet. The terminal must say
   LIVE. Workbench polls the authenticated project-scoped source every five seconds.
7. Send judges the generated `/live/<project>#<share-token>` link, NOT the private
   bridge token. Judges need no account and get only probe readings, device/capture
   identity, last-capture time and offline status. They cannot control hardware.

Both credentials expire in 12 hours. **Revoke bridge and judge link** invalidates
both immediately. Pairing again also rotates both. After ESP32 reboot, pair again
to explicitly reset sequence accounting. An API restart preserves replay state.

## Deployment

Deploy both web and API from this commit. Existing `AUTH_TOKEN_SECRET` must match
between Vercel/Railway; signed-in ownership is required to pair. Keep Railway's
`TELEMETRY_MODE=simulator` for the separate built-in demo: paired projects use their
own serial bridge sources, not the global simulator. No extra secret or serial
port environment variable is needed on Railway. Use the existing single API
replica and persistent SQLite volume; this implementation is not a multi-replica
ingestion service. Vercel must proxy `/api/v1/*` to the API as already configured.

## Evidence and safety

- Original Telemetry v2 passes the existing strict decoder and profile validator.
- Device and wire-profile IDs are bound to the owner-confirmed project. Profile
  edits invalidate pairing. One active project binding per device.
- The server stores unmodified firmware envelopes in `bridge_capture_audit`,
  alongside profile hash and host receipt timestamp. The analysis envelope uses
  the explicitly paired cloud profile ID and host receipt time (ESP32 time may
  be uptime). Every valid window enters the existing measurement/calibration path.
- No Known Good is copied or fabricated: the hosted project learns ten windows
  and still requires explicit healthy confirmation.
- Sequence/uptime rollback, repeated timestamps, stale frames (>5 s), unexpected
  device/profile, schema errors, mismatched probe modes and expired credentials
  are rejected. Offline data is dropped, never queued/replayed.
- This is authenticated provenance from a trusted laptop bridge, not cryptographic
  hardware attestation. A compromised owner/bridge credential can submit fabricated
  input; do not treat this prototype as tamper-proof scientific instrumentation.
- No serial write, PATCH forwarding, electrical output or cloud arming handshake
  is exposed. Physical PATCH remains locked for this transport.
- Judge tokens are distinct, read-only, hashed at rest and sent in a header;
  share links put the token in a URL fragment rather than server access logs.
- Raw audit and measurement storage are bounded by `MAX_MEASUREMENT_WINDOWS`.
  Archive evidence when full; do not silently overwrite it.

## Smoke test

Pair and run bridge → see REAL SERIAL on hosted Workbench → open judge link in an
incognito browser → see changing captures → stop bridge → within five seconds
plus polling, judge page shows offline without readings → revoke → link denied.
Do not unplug target wiring just to validate cloud transport.
