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

## 7. Computer Diagnostics (Layer 10) — DONE (optional for V1, landed anyway)
- [x] Local agent reads CPU/mem/disk/network/process/service state, read-only — `internal/computer` (`PlatformCollector`, `WindowsCollector`, non-Windows stub), plus `Simulator` for demo scenarios
- [x] Deterministic analysis over the snapshot — `internal/computer/rules.go: Analyze`/`validate`/`ValidateExpectations`
- [x] Exposed via `httpapi/computer_handlers.go` (landed on `main` 2026-09-26, commit `3c30fc7`)
- [ ] Not verified: whether it feeds into the same `domain.Evidence`/diagnostic-engine "Combined Mode" described in the original plan, or stays a separate read-only surface — check before building anything on top

## 8. Git / Audit Layer (Layer 11) — DONE
- [x] Security scan on report/files before commit — `internal/gitsync/sync.go: SecretCount`, blocks commit when `SecretScan != "clear"`
- [x] Repo-root validation (rejects paths outside the configured Git worktree, rejects traversal) — `inspectRepo`/`safeTargets`
- [x] Approved → commit (+ optional push via `config.AutoPush`); not approved → local only, no push — `BuildPreview`/`Commit`
- [x] Exposed via `httpapi/git_handlers.go`, opt-in via `Config.Enabled` (default off per security.md), plus a `docs/git-sync.md` writeup (landed on `main` 2026-09-26, commit `0003e79`)
- [x] Diagnostic history/audit trail — `internal/history`, `httpapi/history_handlers.go`, persisted via `store` (commit `f7e74bf`)
- [ ] Fixed today: `inspectRepo` failed on macOS because it compared `filepath.Abs` against git's symlink-resolved `--show-toplevel` without resolving the configured path's own symlinks first (broke under macOS's `/tmp` → `/private/tmp` TMPDIR). See `sync.go: inspectRepo`.

## 9. AI Service Integration — DONE
- [x] Architecture decision made (see `docs/architecture.md`): Vision/CodeAnalysis/ProjectUnderstanding live in Go as swappable adapters, not a separate Python service. `services/probe`, `services/signal-analysis`, `services/vision` stay stub READMEs.
- [x] Vision adapter: Gemini when `GEMINI_API_KEY` set, else `VISION_SKIPPED` — `internal/vision`
- [x] Code analyzer: deterministic parser, Tree-sitter-ready interface — `internal/codeanalysis`
- [x] PROBE (Layer 6): `internal/probe` — `GeminiProvider` rewords the deterministic diagnosis (headline/summary/possible_causes/confidence/next_test JSON, same key-gated swap pattern as vision), capped at the deterministic confidence, never touches measured/spec/baseline evidence. Falls back to `MockProvider` (deterministic passthrough) when no key or on any request/parse failure. Wired in `diagnostics.NewEngineWithProbe`, selected in `cmd/server/main.go`. (2026-09-26)

## 10. Test Planning State (Layer 7, Go half) — DONE
- [x] Stage state machine exists: Diagnose → Test → Repair → Verify (`domain.Stage`)
- [x] Transition endpoints (`/demo/reset`, `/demo/wiggle`, `/demo/repair`)
- [x] Formal test-plan/approve flow — `internal/testplanner`, `httpapi/test_handlers.go`: `POST /tests` plans (recommendation → `domain.DiagnosticWorkflow`), `POST /tests/:id/start` is the persisted user-approval step, `TestLocked` status gates anything requiring PATCH. (landed on `main` 2026-09-26, commit `6528132`)

---

**Next real gap:** WebSocket live push (§1). Everything else in the original ordered list is done.

**When you finish a section, tell me which number — I'll check the code against the doc before you continue.**

---

## Remaining work plan (2026-09-26)

### A. PROBE Gemini adapter (Layer 6) — DONE (2026-09-26)
- [x] `Provider` interface defined — `internal/probe/probe.go` (mirrors `internal/vision`'s swap pattern)
- [x] Request shape: serializes `domain.Evidence` + the deterministic `domain.Diagnosis` into a bounded prompt, instructed not to invent facts
- [x] Response contract: `headline`, `summary`, `possible_causes[]`, `confidence`, `next_test` — structured JSON via `responseSchema`, validated before use (empty headline/summary or out-of-range confidence falls back to deterministic)
- [x] `GeminiProvider` (real, key-gated) + `MockProvider` (deterministic passthrough) when `GEMINI_API_KEY` unset
- [x] Hard rule enforced in code: confidence capped at the deterministic value, never exceeds it; provider only replaces narrative fields, never measured/spec/baseline evidence; cannot trigger PATCH (no such path exists in the interface)
- [x] Unit tests against a local fake Gemini endpoint — `internal/probe/probe_test.go` (reword case, transport-failure fallback, empty-headline fallback), no live paid key in repo
- [x] Wired into `internal/diagnostics/engine.go` (`NewEngineWithProbe`) and selected in `cmd/server/main.go` from the same `GEMINI_API_KEY`/`GEMINI_MODEL` env vars vision uses
- [ ] Not done: prompt tuning against real hardware evidence and a live-key integration test (needs an operator key, out of scope for this pass)

### B. Git / Audit layer (Layer 11) — DONE, landed on `main` outside this plan
- [x] `internal/gitsync` + `httpapi/git_handlers.go` + `internal/history` cover secret-scan-before-commit, opt-in commit/push, and an audit trail. See §8 for the exact commits and today's macOS symlink fix.

### C. Computer Diagnostics (Layer 10) — DONE, landed on `main` outside this plan
- [x] `internal/computer` + `httpapi/computer_handlers.go` cover the read-only local collector and deterministic analysis. See §7 — one thing left to verify: whether it merges into the hardware `domain.Evidence` bundle ("Combined Mode") or stays a separate surface.

### D. WebSocket live telemetry push (Layer 4)
- [ ] `/api/v1/ws/telemetry` endpoint, one connection per active session
- [ ] Push normalized measurement windows as they land, instead of frontend polling `/api/v1/measurements`
- [ ] Keep REST endpoints as-is for initial load / reconnect catch-up
- [ ] Backpressure/close handling if client stalls; no unbounded buffering

### E. Formal "Approve Test Action" endpoint (Layer 7) — DONE, landed on `main` outside this plan
- [x] `internal/testplanner` + `httpapi/test_handlers.go` already implement this: `POST /tests` proposes a plan from a `domain.TestRecommendation`, `POST /tests/:id/start` is the persisted approval step, `TestLocked` status blocks anything requiring PATCH. No further work needed here.

### F. Hardware bench validation (blocks nothing above, but real gap)
- [ ] Flash firmware to an actual ESP32 + HC-SR04, verify P1-P6 readings against a multimeter/scope
- [ ] Confirm ADC safety bounds (never >3.3V) hold on real rail noise, not just simulated frames
- [ ] Validate serial reconnect/timeout behavior against a real USB disconnect, not just mocked source

### G. Deferred / explicitly not doing yet
- Real PATCH validator (Layer 8) — stays locked (`423 PATCH_LOCKED`) until hardware output is intentionally re-enabled; do not build early
- Auth / multi-user / device identity — no plan yet, needed before any production exposure
- Wi-Fi/WebSocket/MQTT telemetry transports — USB serial only for now
- Multi-file project uploads, video, GitHub import, OCR — single image + single source payload only

**Suggested build order:** D → F, with G staying parked. (A, B, C, E all done 2026-09-26 — see §7-§10; most of B/C/E landed independently from other work on `main` while this plan was being executed.) D is the only remaining code gap and finishes the interaction loop; F is hardware time, not code time, and can run in parallel once a board is available.
