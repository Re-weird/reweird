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
- [x] **Real multi-user auth**, not a placeholder. Users log in; each user
  only sees their own projects.

## Open — needs a decision before building

- [ ] Session strategy: cookie+server-side session vs JWT
- [ ] Signup flow: open signup, invite-only, or single-admin-seeds-users?
- [ ] Password requirements / hashing lib (Go: bcrypt/argon2)
- [ ] What happens to existing un-owned projects already in the DB (migrate
  to a default user, or wipe dev data)?
- [ ] Dashboard project card content: name, controller, last diagnosis
  status, last activity time — confirm exact fields
- [x] Per-project page structure: **real Next.js routes**,
  `/projects/[id]` + 9 sub-routes, each a real URL with its own
  `page.tsx`. Superseded the earlier view-state tab-bar approach.
- [x] Left sidebar removed entirely, app-wide — replaced by a single
  top nav (`global-nav.tsx`: logo, Projects/Account/Settings) plus the
  per-project tab bar rendered by `app/projects/[id]/layout.tsx`.
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
