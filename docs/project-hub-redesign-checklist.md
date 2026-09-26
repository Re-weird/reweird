# Project Hub Redesign — Checklist

Living doc. Keep adding decisions as they're finalized. Build starts once this
is stable — items get checked off during implementation, not before.

---

## Finalized decisions

- [x] **Dashboard = project hub, not a working diagnostic surface.** GitHub-style:
  list of projects, create new project. No live probe cards, no charts on
  the dashboard itself.
- [x] **Everything else is per-project.** Live signals, diagnosis, PROBE,
  guided tests, VERIFY, computer diagnostics, git sync, history/reports —
  all live inside a project's own pages, reached from the dashboard.
- [x] **Real multi-user auth** is the decided direction (users log in; each
  user only sees their own projects). *Decided, not built:* there is no
  sign-in yet. Project visibility (public/private) is stored and shown but
  not enforced until auth exists. The `frontend` branch has Clerk/Google
  sign-in work that is not on `main` yet.

- [x] **Database: PostgreSQL** for users and all app data (projects,
  profiles, diagnostic sessions, measurement windows, test runs, reports,
  git events). SQLite stays for local dev and tests. The switch is a new
  `domain.Repository` implementation in `apps/api/internal/store`; domain
  and HTTP code do not change. Replaces the "MongoDB Atlas later" note in
  `README.md` / `docs/architecture.md`.
- [x] **Git log is per project.** A project can link a repo; its Git log
  tab shows ReWeird's own report commits plus the repo's commits, each
  lined up with the diagnostic sessions before and after it.

## Open — needs a decision before building

- [ ] Session strategy: cookie+server-side session vs JWT
- [ ] Signup flow: open signup, invite-only, or single-admin-seeds-users?
- [ ] Password requirements / hashing lib (Go: bcrypt/argon2)
- [ ] What happens to existing un-owned projects already in the DB (migrate
  to a default user, or wipe dev data)?
- [ ] Dashboard project card content: name, controller, last diagnosis
  status, last activity time — confirm exact fields
- [x] Per-project page structure: **real Next.js routes**,
  `/projects/[id]` + 10 sub-routes (including `passport`, from main), each
  a real URL with its own
  `page.tsx`. Superseded the earlier view-state tab-bar approach.
- [x] Left sidebar removed. Top bar: logo, breadcrumb, actions
  (`global-nav.tsx`). On Dashboard/Projects/Settings a fixed profile rail
  sits on the left with the tab row beside it (`section-nav.tsx`); inside a
  project the rail is hidden and the same tab row shows the project tabs.
- [ ] Combined Mode and PATCH have no UI yet (backend not ready) — confirm
  they stay out of scope for this redesign

---

## BACKEND

### Auth (new)
- [ ] `users` table migration: id, email, password_hash, created_at
- [ ] Signup endpoint
- [ ] Login endpoint (issues session/token per decision above)
- [ ] Logout endpoint
- [ ] Auth middleware on all `/api/v1/*` routes
- [ ] `owner_id` column on `projects` table + migration
- [ ] Scope every project-related endpoint to the authenticated user's
  `owner_id` (list, get, media, code, analyze, profile, probe-plan, tests,
  history, reports, git — anything keyed by project id)
- [ ] Decide + implement handling of existing unowned dev-data projects

### PostgreSQL (new)
- [ ] Postgres service in `docker-compose.yml`; `DATABASE_URL` env var
  (SQLite used when unset)
- [ ] Migrations tool (e.g. goose or golang-migrate) and initial schema:
  `users`, `projects`, `project_profiles`, `probe_plans`,
  `diagnostic_sessions`, `measurement_windows`, `known_good_baselines`,
  `test_workflows`, `reports`, `git_events`
- [ ] `users`: id, email (unique), name, avatar_url, auth_provider,
  provider_subject, created_at, last_seen_at
- [ ] `owner_id` FK on `projects`; `project_id` FK on every
  session/window/baseline/workflow/report row (today these are one shared
  instance for everybody, not per project)
- [ ] Postgres `Repository` implementation passing the existing store tests
- [ ] One-time import of the dev SQLite file (or wipe; see open question)

### Git log (new)
- [ ] `projects.repo_url` + `default_branch` (link a repo to a project)
- [ ] `git_events` table: project_id, sha, short_sha, branch, message,
  author_name, author_email, committed_at, source (`reweird_report` |
  `repo`), files_changed, additions, deletions, report_id (nullable),
  session_before_id / session_after_id (nullable), pushed (bool),
  push_error (nullable), created_at
- [ ] Record a `git_events` row every time git sync commits/pushes a report
- [ ] Import the linked repo's commits (local `git log` for a local repo;
  GitHub API later) and dedupe by sha
- [ ] `GET /api/v1/projects/:id/git-log?cursor=&source=&branch=`
- [ ] Owner check on all of the above

### Existing (already built, just needs project-scoping wired through)
- [x] `GET/POST /api/v1/projects`, `GET /api/v1/projects/:id`
- [x] Project media/code upload + analyze
- [x] Project profile get/update/confirm
- [x] Probe plan get/confirm
- [x] Diagnostics, PROBE, tests, VERIFY, computer diagnostics, git sync,
  history/reports — all already project- or session-scoped at the data
  level, just need the owner check added

## FRONTEND

### New
- [ ] Login page
- [ ] Signup page
- [ ] Auth state (logged in/out) + redirect-to-login guard on all routes
- [ ] Logout action
- [x] Dashboard rebuilt as project list ("my projects") + "new project"
  action — `app/project-dashboard.tsx`, added as a "Projects" nav entry
  next to Workbench (not yet a replacement for Workbench itself — both
  exist; still no auth to scope the list per-user)
- [x] Account-level activity dashboard — `app/account-dashboard.tsx`,
  now the root route `/` (the true landing page). Every grid/value
  (heatmap, activity-overview chart, session outcomes) renders
  neutral/zero placement, not "no data yet" text — real number wiring
  is still a separate follow-up task. Unboxed styling: no card borders
  on the sidebar/sections, just hairline dividers.
- [x] Removed the three redundant "add a project" CTAs (topbar, old
  sidebar Workspace group, Workbench's own upload panel) - Project
  Dashboard is now the one place to create a project
- [ ] Basic account settings (at minimum: logout, maybe email display)
- [ ] Profile rail and account dashboard read real numbers from `users` +
  per-user aggregates (projects, sessions, joined date, heatmap)
- [ ] Project "Git log" tab: commit list (short sha, message, author,
  relative time, branch, +/- stats, pushed/local badge), each row linked
  to its report and to the verdict before/after (e.g. FAIL -> PASS);
  filter by source and branch; empty state with "Link a repo"

### Restructure existing (moved under real /projects/[id]/* routes)
- [x] Live/probes view — folded directly into Workbench instead of
  staying a separate tab (probe grid + signal chart + rule engine)
- [x] Diagnosis view, Guided test view, Computer diagnostics view,
  History/reports view + git-sync panel, Probe plan view, Overview
  (profile) view — all now real routes under `/projects/[id]/*`
- [x] Real per-project routing landed: `app/projects/[id]/layout.tsx`
  loads whichever project the URL names (or clears to demo mode for
  the placeholder id `demo`) - no more single shared in-memory
  "current project"; opening a different URL loads a different project
- [ ] Project workflow (upload/profile/confirm) — still a modal
  (`NewProjectModal`, global, triggered from the nav or Project
  Dashboard); not yet a project's own onboarding page

---

## Explicitly out of scope for this pass

- Video upload (backend doesn't support it yet)
- Combined Mode UI (backend evidence not wired yet)
- PATCH UI (locked, no hardware output path yet)
- Hardware bench validation (separate physical-layer work, unrelated to
  this redesign)
