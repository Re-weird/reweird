# Physical Git

Physical Git is version control for the physical/electrical state of a hardware project — not for its source code. The intended workflow is linear: **COMMIT → HISTORY → DIFF → RESTORE → VERIFY**. This milestone builds only COMMIT (create a Physical Commit and list/fetch it back); History/Diff/Restore/Verify, branches, merges, and checkout are not implemented. There is no parent-id graph — a project's physical history is a single ordered sequence, numbered `HW-001`, `HW-002`, … per project.

GitHub code integration (**CONNECT GITHUB → FETCH CODE → ANALYZE**, associating a firmware revision with a Physical Commit) is a separate, later feature. `PhysicalCommit` reserves `software_provider`/`software_repository`/`software_revision`/`software_analysis` fields for it, but they stay null until that milestone ships. Do not confuse the two: this repo also has an unrelated `git sync` feature (see [git-sync.md](git-sync.md)) that pushes diagnostic reports to a GitHub repository — Physical Git shares no code or data with it.

## Evidence sources

A Physical Commit is a snapshot of whatever ReWeird already knows about a project at commit time, drawn from these sources:

| Source | What it captures | Physical Commit field |
| --- | --- | --- |
| Camera | Visual state — a raw JPEG/PNG photo | `image` (uploaded fallback for now; a live capture abstraction can be added later without changing this field) |
| Circuit Map | Declared structural state — components, connections, probes | `profile_snapshot` (full copy of `ProjectProfile` at commit time) |
| Device Passport | Component knowledge / physical known-good baseline, if one exists | `passport_baseline_ids` (reference to an already-immutable `KnownGoodBaseline` row, never copied) |
| ESP32 probes | Electrical state — the most recent valid measurement | `measurement_id` (reference to an already-immutable `MeasurementWindow` row, never copied) |
| Code | Software intent/state (future GitHub integration) | `software_provider`/`software_repository`/`software_revision`/`software_analysis` (reserved, null in V1) |

Every source above is independent and optional. A normal project with no Device Passport, no known-good baseline, and no telemetry yet still produces a valid commit — it just has fewer populated fields. Nothing is ever fabricated to fill a gap: if real evidence is unavailable, the corresponding field is simply absent, not a placeholder or a synthetic substitute. In particular, Physical Git never touches the frontend's synthetic/demo session data, and a real commit auto-attaches only a **physical** known-good baseline, never a **simulated** one — simulated baselines belong to the demo/game workflow, not real Physical Git history.

## Snapshot semantics

Mutable "current state" (`ProjectProfile`, `KnownGoodBaseline`) is handled two ways so that editing the live project never retroactively changes what a past commit says:

- **Copied by value**: `profile_snapshot` is a full copy of the `ProjectProfile` at commit time (this is exactly the data the Circuit Map view renders).
- **Referenced by immutable id**: `passport_baseline_ids` and `measurement_id` point at rows that are already append-only/immutable once saved (`known_good_baselines`, `measurement_windows`), so referencing them by id is safe — those rows are never rewritten or deleted in bulk.

## Vision (planned, not built this milestone)

`image` is raw evidence, not an interpretation. Creating a Physical Commit never calls Gemini — a commit must succeed even when Gemini is unavailable, no API key is configured, or vision analysis has never run. A later milestone will add an explicit, user-triggered "Analyze Hardware" action that runs a *saved* commit's image through ReWeird's existing Gemini Vision / `internal/projectunderstanding` pipeline (the same one project analysis already uses) and stores the structured result keyed by the commit's id — reusing that infrastructure rather than building a second vision system, and without changing what the raw `image` field means.

**The LLM reasons about evidence. It does not create evidence.**
