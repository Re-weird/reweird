# Physical Git

Physical Git is version control for the physical/electrical state of a hardware project — not for its source code. The workflow is linear: **COMMIT → HISTORY → DIFF → RESTORE → VERIFY**, and all five stages are implemented. There are no branches, no merges, and no checkout — there is no parent-id graph at all. A project's physical history is a single ordered sequence, numbered `HW-001`, `HW-002`, … per project, and it stays that way forever: Restore never rewrites an old commit, and "returning to an earlier state" is recorded as a brand-new commit later in the same sequence (e.g. `HW-008` noting "Restored toward HW-003"), never as a branch or a rewound HEAD.

GitHub code integration (**CONNECT GITHUB → FETCH CODE → ANALYZE**, associating a firmware revision with a Physical Commit) is a separate, later feature. `PhysicalCommit` reserves `software_provider`/`software_repository`/`software_revision`/`software_analysis` fields for it, but they stay null until that milestone ships. Do not confuse the two: this repo also has an unrelated `git sync` feature (see [git-sync.md](git-sync.md)) that pushes diagnostic reports to a GitHub repository — Physical Git shares no code or data with it.

## Evidence sources

A Physical Commit is a snapshot of whatever ReWeird already knows about a project at commit time, drawn from these sources:

| Source | What it captures | Physical Commit field |
| --- | --- | --- |
| Camera | Visual state — a raw JPEG/PNG photo | `image` (manual upload, or one real frame auto-captured from a configured MJPEG camera — see [Vision camera](#vision-camera-mjpeg) below; both write the same field the same way) |
| Circuit Map | Declared structural state — components, connections, probes | `profile_snapshot` (full copy of `ProjectProfile` at commit time) |
| Device Passport | Component knowledge / physical known-good baseline, if one exists | `passport_baseline_ids` (reference to an already-immutable `KnownGoodBaseline` row, never copied) |
| ESP32 probes | Electrical state — the most recent valid measurement | `measurement_id` (reference to an already-immutable `MeasurementWindow` row, never copied) |
| Code | Software intent/state (future GitHub integration) | `software_provider`/`software_repository`/`software_revision`/`software_analysis` (reserved, null in V1) |

Every source above is independent and optional. A normal project with no Device Passport, no known-good baseline, and no telemetry yet still produces a valid commit — it just has fewer populated fields. Nothing is ever fabricated to fill a gap: if real evidence is unavailable, the corresponding field is simply absent, not a placeholder or a synthetic substitute. In particular, Physical Git never touches the frontend's synthetic/demo session data, and a real commit auto-attaches only a **physical** known-good baseline, never a **simulated** one — simulated baselines belong to the demo/game workflow, not real Physical Git history.

## Snapshot semantics

Mutable "current state" (`ProjectProfile`, `KnownGoodBaseline`) is handled two ways so that editing the live project never retroactively changes what a past commit says:

- **Copied by value**: `profile_snapshot` is a full copy of the `ProjectProfile` at commit time (this is exactly the data the Circuit Map view renders).
- **Referenced by immutable id**: `passport_baseline_ids` and `measurement_id` point at rows that are already append-only/immutable once saved (`known_good_baselines`, `measurement_windows`), so referencing them by id is safe — those rows are never rewritten or deleted in bulk.

## Vision

`image` is raw evidence, not an interpretation. Creating, listing, fetching, or diffing a Physical Commit never calls Gemini — a commit must succeed even when Gemini is unavailable, no API key is configured, or vision analysis has never run. An explicit, user-triggered **Analyze Hardware** action (`POST .../physical-commits/:commitId/analyze-hardware`) runs a *saved* commit's image through ReWeird's existing Gemini Vision / `internal/projectunderstanding` pipeline (the same one project analysis already uses) and stores the structured result (`domain.VisionAnalysis`) keyed by the commit's id, in a separate `physical_commit_vision_analyses` table — reusing that infrastructure rather than building a second vision system, and without changing what the raw `image` field means, mutating `ProjectProfile`/Circuit Map/Device Passport, or touching the commit itself. Only a genuinely successful (`VISION_COMPLETE`) result is ever persisted; a skipped (Gemini not configured) or failed attempt is rejected as a request error and never overwrites a prior successful analysis. Re-analyzing a commit replaces its interpretation record — the underlying `PhysicalCommit` stays immutable throughout.

**The LLM reasons about evidence. It does not create evidence.**

## Vision camera (MJPEG)

A project can optionally configure a real MJPEG camera — an Android phone running an "IP Webcam"-style app on the same Wi-Fi as the backend is the supported hackathon setup — as its source of raw visual evidence, instead of (or alongside) manually uploading a photo. The config is `Project.camera_config` (`{source_type: "mjpeg", url}`), stored on the project itself and managed through `PUT`/`DELETE .../projects/:id/camera-config`, the same non-content-update convention as `visibility`.

`internal/cameracapture` is the one MJPEG client: it connects to the configured URL, extracts **exactly one** complete frame (parsing a `multipart/x-mixed-replace` stream, or accepting a plain single-image response for cameras that serve a still directly), validates that the frame actually decodes as an image, and closes the connection immediately — it never holds a stream open longer than it takes to get that one frame, and it never sends anything to Gemini. Two read-only actions let a user confirm the camera before relying on it: `POST .../camera/test` (a real connection attempt, reporting `CONNECTED`/`UNREACHABLE`/`INVALID_STREAM`/`TIMEOUT`/`NOT_CONFIGURED` — never `CONNECTED` unless a frame actually decoded) and `POST .../camera/capture-test-frame` (returns one frame inline, base64-encoded, for a positioning preview). Neither ever creates a Physical Commit or persists anything.

**Physical Commit integration**: `createPhysicalCommit` uses the camera automatically only when the request has no manually attached photo and the project has a `camera_config` — a manual upload always wins. Any camera failure (not configured, unreachable, timeout, invalid stream, or the frame failing to save) never blocks the commit; it simply proceeds with no image, exactly like a project with no camera at all, and never affects the Circuit Map/measurement/passport evidence already gathered for that commit. The captured frame is written through the exact same `projects.SavePhysicalCommitImage` used by manual upload, so once it lands in `HW-004` it is exactly as immutable as any other commit's image — a later camera frame becomes `HW-005`'s evidence, never `HW-004`'s.

**Analyze Hardware always analyzes the saved frame.** Nothing changed in `analyzePhysicalCommitHardware` for this feature: it resolves `commit.Image.StorageRef` and nothing else, so it is structurally incapable of fetching a new live frame from the camera — whether `image` came from a manual upload or a camera capture is invisible to it.

**This is a LAN camera feature, not a generic URL fetcher.** `cameracapture.ValidateURL` requires `http`/`https` and rejects embedded credentials, but deliberately does not block private/RFC1918 addresses — a real camera on the same Wi-Fi as the backend is expected to be at an address like `10.x.x.x`. What keeps this narrow instead of an open SSRF primitive: every request is bounded (an 8s timeout, an 8 MB frame cap), the response must actually decode as an image or a bounded connectivity report (never arbitrary bytes returned to the caller), and both `test`/`capture-test-frame` only ever operate on the URL of a project the caller can already access (ownership-checked, the same as every other project endpoint) — there is no separate "fetch this arbitrary URL" endpoint.

**Local hardware vs. the hosted demo.** A cloud-hosted backend cannot reach a private LAN address like a phone's `10.x.x.x` IP — this camera feature is for a local `go run ./cmd/server` reachable by the camera's Wi-Fi, not the public deployment. The seeded, cloud-reachable `physical-git-demo` project (see below) never has a `camera_config` and is unaffected either way.

## Diff: raw visual evidence vs. semantic (AI-interpreted) visual evidence

`GET .../physical-commits/diff?from=...&to=...` returns two independent layers under `visual` and `semantic_visual` that answer different questions and are never forced to agree:

- **`visual`** (raw visual diff) — did the captured image bytes change? Computed from `ProjectMedia.SHA256` alone. A camera angle change reports `CHANGED` here even if the same hardware is in frame.
- **`semantic_visual`** (semantic visual diff) — do the two commits' already-stored Gemini Vision interpretations report the same detected components, by identity and count? Computed entirely from whatever `PhysicalCommitVisionAnalysis` rows already exist for `from`/`to` — **this request never calls Gemini**, it only compares previously-persisted results. Component identity is `CatalogID` when Gemini/the Component Catalog already resolved one, else a case/whitespace-normalized name; two unrelated names are never guessed to be the same component. Confidence, warnings, model metadata, and `Relationships` (wiring claims) are deliberately excluded from this comparison — they describe interpretation quality, not physical state. If neither commit has been analyzed, the status is `NOT_CAPTURED`; if only one has, it's `UNAVAILABLE` (with `from_analyzed`/`to_analyzed` telling the frontend which side is missing) — never a guessed or partial comparison.

Because vision analysis is a replaceable interpretation, not immutable evidence, `semantic_visual` reflects whatever is currently stored — re-analyzing one commit can change a diff's semantic section on a later request without changing the underlying `PhysicalCommit`, its `image`, or any other evidence field.

Diff responses also carry a small, bounded `diagnostic_context` array (max 8 lines): plain factual observations like `"HW-006 -> HW-007: ECHO connection changed (CHANGED)."`, derived deterministically from the sections above. It exists so a future PROBE prompt could cite a couple of relevant historical facts without ever receiving a project's entire Physical Git history — and it never states or implies causation.

## Restore

`GET .../physical-commits/:commitId/restore?source=<commitID>` builds a deterministic checklist for returning the project's *observable* state to `commitId` (the target). It never touches hardware and never calls Gemini — it's computed the same way Diff is, reusing Diff's own comparison helpers (component/connection field-diffing, vision identity grouping) instead of a second diff engine.

`source` is the current/reference commit to compare the target against; when omitted, it defaults to the project's newest commit other than the target. If no reference commit exists at all (e.g. the target is the only commit in the project), every section reports `UNAVAILABLE` rather than inventing a "current state" — restoration guidance requires something to compare against.

Each of the five sections (`components`, `circuit`, `electrical`, `visual`, `software`) carries its own status (`MATCH` / `ACTION_REQUIRED` / `VERIFY_REQUIRED` / `NOT_CAPTURED` / `UNAVAILABLE`) and a list of checklist actions. Two rules keep this honest:

- **Electrical differences are never `ACTION_REQUIRED`.** Voltage/timing/activity aren't something a user can directly set — they can only be re-measured, so this section is always `VERIFY_REQUIRED` whenever the target captured a measurement.
- **Visual differences (raw bytes or AI-detected components) are never `ACTION_REQUIRED` either.** A byte-level image difference or an AI-detected component-count difference is suggestive, not proof, so this section only ever reaches `MATCH`/`VERIFY_REQUIRED`/`NOT_CAPTURED`/`UNAVAILABLE`. AI-derived actions are marked `ai_interpreted: true` and worded as unconfirmed interpretation, never presented the same way as a raw-evidence action.

Software actions (when a commit ever has software fields populated — it doesn't yet, see below) describe a manual step ("Manually align software_revision with HW-003's captured reference...") — Restore never performs a Git checkout.

## Verify

`GET .../physical-commits/:commitId/verify?observed=<commitID>` deterministically asks whether newly observed evidence supports having restored the project to `commitId`'s captured state. It is evidence-based — there is no "mark as done" button that flips this to true.

Verify's "current observation" is resolved the same way a new Physical Commit's evidence is resolved: the project's **live** current `ProjectProfile` (not a commit snapshot — Verify checks the live Circuit Map, not what some other commit happened to capture), the most recently captured `MeasurementWindow`, and (since this app has no standalone "verification photo" mechanism) a reference commit's image/persisted vision analysis, defaulting to the newest other commit the same way Restore's `source` does.

- **Structural verification** (`components`, `circuit`) reuses the same deterministic diff helpers as Physical Diff: `UNCHANGED` → `SUPPORTED`, `CHANGED` → `NOT_SUPPORTED`, `NOT_CAPTURED`/`UNAVAILABLE` pass through unchanged.
- **Electrical verification** reuses `internal/testplanner`'s existing deterministic profile-limit comparison (`Planner.Verify`) rather than a second, competing algorithm — the target's measurement stands in as the "known-good reference" side of that comparison. No tolerance is invented here; it's entirely testplanner's own severity comparison against the confirmed Project Profile's configured limits.
- **Visual verification is deliberately conservative and can never return `NOT_SUPPORTED`** — raw image bytes differing, or an AI-detected component count differing, can only downgrade the result to `INCONCLUSIVE`, never flip it to a contradiction.
- **Software verification** is `NOT_CAPTURED` today (Physical Git's GitHub/software integration is a separate, later milestone) and never blocks the overall result.

The `overall` status is derived deterministically from the five categories: any category `NOT_SUPPORTED` makes the overall result `NOT_SUPPORTED`; otherwise, at least one category `SUPPORTED` (with none contradicting) makes it `SUPPORTED` — categories that are `INCONCLUSIVE`/`NOT_CAPTURED`/`UNAVAILABLE` never block that; otherwise it's `INCONCLUSIVE`. This matches the product rule: partial evidence can still say "supported by available evidence," but nothing is ever claimed as "exactly restored."

## Save the restored state

After Verify, the frontend's "Commit restored state" button is just the existing **Commit Physical State** action with a prefilled note ("Restored toward HW-003") — a normal new linear commit. The target commit is never mutated, and there is no parent/branch link recorded; the history simply continues (`HW-001, HW-002, ..., HW-008`).

## What's still not implemented

Physical Git's GitHub/software integration (`software_provider`/`software_repository`/`software_revision`/`software_analysis`) remains a separate, later milestone — Restore and Verify both work completely without it (`software` always reports `NOT_CAPTURED` today), and neither Restore nor Verify implements a Git checkout of any kind. Physical Git also still has no branches, merges, or checkout of any kind, and Restore never automatically modifies hardware, ProjectProfile, Circuit Map, or Device Passport — it only ever describes what a human should change.
