# Backend (Go) Build Checklist

Scope: everything Go owns per context.md (Layer 4, Go-half of Layer 5, Layer 8, 9, 10, 11).
Check items off as you go. Ping me when a section is done — I'll review before you move on.

> **2026-09-26 correction:** first pass of this checklist was written from a shallow
> file scan and was wrong in places. Re-audited against actual code + `docs/architecture.md`
> + `docs/security.md`. Sections 1-4 and most of 9 were already built. Updated below.

---

## 1. Core API / Data Layer (Layer 4) — DONE
- [x] `cmd/server` boots Fiber app, config loaded
- [x] SQLite wired (`store/sqlite.go`, Repository interface)
- [x] Projects CRUD — `projects/`
- [x] Project Profiles store + fetch — `profiles/`
- [x] Component Catalog loaded into Go — `packages/component-catalog`, wired in `cmd/server/main.go`
- [x] Measurement/session storage (`SaveSession`/`LatestSession`)
- [ ] WebSocket endpoint pushing live telemetry to frontend (still polling-based; not yet built)

## 2. Device / Serial Layer (Layer 3 hookup) — DONE
- [x] Serial connection to ESP32 — `internal/transport/serialsource`
- [x] Parse + validate incoming telemetry JSON — `internal/telemetry/validate.go`
- [x] Reject malformed/out-of-range/unconfirmed-profile messages
- [x] Device status via `/api/v1/telemetry/status`

## 3. Signal Analysis (Go side, Layer 5) — DONE
- [x] Voltage min/max, HIGH/LOW, frequency, PWM duty, jitter, dropout count, stability — `internal/signalanalysis`
- [x] Cross-probe relationship checks (simultaneous dropout groups)
- [x] Structured "signal facts" (`domain.DerivedFacts`), not raw sample dumps

## 4. Diagnostic Engine — DONE
- [x] Rules evidence source — `internal/diagnostics/engine.go: evaluateRules`
- [x] Specs evidence source (vs Catalog / configured expected signal)
- [x] Baseline evidence source (only when `Baseline.Status.Trusted()`)
- [x] Software-vs-Hardware style mismatch checks (movement correlation, dropout deltas)
- [x] Evidence Builder → structured `domain.Evidence` JSON
- [x] Falls back to low-confidence/"no rule fired" diagnosis when evidence is weak (no hard UNKNOWN enum — acceptable for V1)

## 5. Safety / Control Layer (Layer 8) — INTENTIONALLY DEFERRED
- [x] `/api/v1/patch` exists and returns `423 PATCH_LOCKED` on purpose
- [x] Firmware has no PATCH output path (see `docs/security.md`)
- [ ] Real PATCH validator (pin/signal/voltage/duration/limits) — **do not build until hardware output is intentionally re-enabled**; see `docs/security.md` "Required before real hardware"

## 6. VERIFY / Report Layer (Layer 9)
- [x] Snapshot "before" measurements (`domain.Session.Before`)
- [x] Re-measure + compare on `StageVerify` (`domain.Session.After`)
- [x] Compare before/after → fixed / still_failing / unclear — `internal/reports` (**done today**)
- [x] Report endpoint `GET /api/v1/report` (**done today**)

## 7. Computer Diagnostics (Layer 10) — NOT STARTED (optional for V1 per context.md)
- [ ] Local agent reads CPU/mem/disk/network/process/service state
- [ ] Feed results into same Diagnostic Engine (Combined Mode)

## 8. Git / Audit Layer (Layer 11) — NOT STARTED, real gap
- [ ] Security scan on report/files before commit
- [ ] Determine commit author: Human vs "ReWire Bot"
- [ ] Approved → auto commit + auto push
- [ ] Not approved → local only, no push
- [ ] Audit log of every diagnostic/test/commit action

## 9. AI Service Integration — DECIDED + MOSTLY DONE
- [x] Architecture decision made (see `docs/architecture.md`): Vision/CodeAnalysis/ProjectUnderstanding live in Go as swappable adapters, not a separate Python service. `services/probe`, `services/signal-analysis`, `services/vision` stay stub READMEs.
- [x] Vision adapter: Gemini when `GEMINI_API_KEY` set, else `VISION_SKIPPED` — `internal/vision`
- [x] Code analyzer: deterministic parser, Tree-sitter-ready interface — `internal/codeanalysis`
- [ ] PROBE (Layer 6): currently a **deterministic mock** inside the diagnostic engine, per `docs/architecture.md`. Real Gemini PROBE adapter (finding/confidence/evidence_ids/next_test JSON, same swap pattern as vision) not built yet.

## 10. Test Planning State (Layer 7, Go half) — PARTIAL
- [x] Stage state machine exists: Diagnose → Test → Repair → Verify (`domain.Stage`)
- [x] Transition endpoints (`/demo/reset`, `/demo/wiggle`, `/demo/repair`)
- [ ] Formal "Approve Test Action" endpoint/flag distinct from the demo transition routes

---

**Next real gaps, in order:** PROBE Gemini adapter (§9) → Git/Audit layer (§8) → Computer Diagnostics (§7, optional) → WebSocket live push (§1) → formal Approve Test Action (§10).

**When you finish a section, tell me which number — I'll check the code against the doc before you continue.**

---

## Remaining work plan (2026-09-26)

### A. PROBE Gemini adapter (Layer 6) — highest priority gap
- [ ] Define `PROBEProvider` interface (mirrors existing `VisionProvider` swap pattern in `internal/vision`)
- [ ] Request shape: serialize `domain.Evidence` (rules + specs + baseline + software facts, each tagged with provenance) into a bounded prompt payload
- [ ] Response contract: `finding`, `confidence`, `evidence_ids[]`, `next_test` — structured JSON, validated before use (same discipline as vision's structured-response parser)
- [ ] `GeminiProbeProvider` (real, key-gated) + keep current deterministic mock as `FakeProbeProvider` fallback when `GEMINI_API_KEY` unset
- [ ] Hard rule: PROBE output is interpretation only — cannot write measured/spec/baseline evidence fields, cannot trigger PATCH
- [ ] Unit tests against a local fake Gemini endpoint (same pattern as existing vision tests), no live paid key in repo
- [ ] Wire into `internal/diagnostics/engine.go` as the source of the human-readable diagnosis text currently hardcoded/templated

### B. Git / Audit layer (Layer 11) — real gap, not started
- [ ] Secret-scan pass over report/session data before any commit (block on match, no bypass)
- [ ] Commit author resolution: real human identity when available, else a bot identity — never silently attribute AI output to a person
- [ ] Opt-in adapter: approved → commit + push; not approved → local-only commit, no push
- [ ] Append-only audit log: one record per diagnostic/test/repair/commit action, with timestamp + actor + evidence refs
- [ ] User-facing setting to enable/disable git sync per project (default OFF, per current security.md stance)

### C. Computer Diagnostics (Layer 10) — optional for V1
- [ ] Local agent: read CPU/mem/disk/network/process/service state (read-only, no control path)
- [ ] Normalize into the same `domain.DerivedFacts` shape signal-analysis already produces, so diagnostic engine rules apply unmodified
- [ ] "Combined Mode": diagnostic engine consumes hardware evidence + computer evidence together in one Evidence bundle
- [ ] Explicitly out of scope: any write/kill/service-restart action — read-only until a separate PATCH-style gate exists

### D. WebSocket live telemetry push (Layer 4)
- [ ] `/api/v1/ws/telemetry` endpoint, one connection per active session
- [ ] Push normalized measurement windows as they land, instead of frontend polling `/api/v1/measurements`
- [ ] Keep REST endpoints as-is for initial load / reconnect catch-up
- [ ] Backpressure/close handling if client stalls; no unbounded buffering

### E. Formal "Approve Test Action" endpoint (Layer 7)
- [ ] Replace implicit demo-transition routes (`/demo/wiggle`, `/demo/repair`) with a generic `POST /api/v1/projects/:id/test-actions/:actionId/approve`
- [ ] Test planner proposes an action (from allowed action set) + evidence justification; user approval is a separate persisted step before execution
- [ ] Keeps the same user-in-the-loop gate PATCH already has, applied to non-hardware test actions too

### F. Hardware bench validation (blocks nothing above, but real gap)
- [ ] Flash firmware to an actual ESP32 + HC-SR04, verify P1-P6 readings against a multimeter/scope
- [ ] Confirm ADC safety bounds (never >3.3V) hold on real rail noise, not just simulated frames
- [ ] Validate serial reconnect/timeout behavior against a real USB disconnect, not just mocked source

### G. Deferred / explicitly not doing yet
- Real PATCH validator (Layer 8) — stays locked (`423 PATCH_LOCKED`) until hardware output is intentionally re-enabled; do not build early
- Auth / multi-user / device identity — no plan yet, needed before any production exposure
- Wi-Fi/WebSocket/MQTT telemetry transports — USB serial only for now
- Multi-file project uploads, video, GitHub import, OCR — single image + single source payload only

**Suggested build order:** A → D → E → B → C → F, with G staying parked. A unlocks the story's "AI explains it" step; D+E finish the interaction loop; B closes the named "real gap"; C is optional; F is hardware time, not code time, and can run in parallel with any of the above once a board is available.
