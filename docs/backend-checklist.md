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
