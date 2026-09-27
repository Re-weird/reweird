# Database architecture

ReWeird's diagnostic system is authoritative in SQLite (`apps/api/internal/store`)
and stays that way. This milestone adds two new, purely additive persistence
responsibilities alongside it — neither replaces nor migrates any existing
diagnostic data:

```
                     ReWeird
                        |
           +------------+------------+
           |                         |
       PRODUCT DATA             DIAGNOSTICS
           |                         |
           v                         v
     MongoDB Atlas               SQLite
     -------------               ------
     Users                       Projects
     Equipment                   Project Profiles
     Project<->Equipment         Sessions / Workflows
                                 Baselines / Reports
                                     |
ESP32 --- electrical telemetry ------+
                                     v
                              Tiger Data
                              ----------
                              Flattened per-probe
                              measurement history
```

Both stores are entirely optional. With neither `MONGODB_URI` nor
`TIGER_DATABASE_URL` configured, the API boots exactly as it did before this
milestone: SQLite-backed diagnostics, Demo Mode, and the simulator all work
unchanged. The new endpoints degrade to a clear `503` instead of silently
no-op-ing or fabricating data.

## MongoDB: product data (`apps/api/internal/productdata`)

Answers "what does this user own?" — never electrical history, and never a
copy of the trusted Component Catalog.

- **`users`**: `{ _id, auth_provider, auth_subject, email?, display_name?,
  avatar_url?, created_at_ms, updated_at_ms }`. Unique index on
  `(auth_provider, auth_subject)`. Created/looked up only by `ResolveUser`,
  which is only ever called with the server-verified subject claim from
  `apps/web`'s signed bearer token (`GET /api/v1/me`) — never anything a
  request claims about itself. `email`/`display_name` are filled in once,
  from that same verified token's custom claims, and never overwritten by a
  later, possibly-stale hint.
- **`equipment`**: `{ _id, owner_id, catalog_id?, name, category?,
  manufacturer?, model?, serial_number?, controller_type?, logic_voltage?,
  notes?, created_at_ms, updated_at_ms }`. Index on `owner_id`. `catalog_id`
  is validated against `packages/component-catalog` when supplied and may be
  `null` for custom/unknown equipment — Equipment only ever *references* a
  catalog entry, never copies its trusted specification data.
- **`project_equipment`**: `{ project_id, equipment_id, owner_id, role?,
  created_at_ms }`. Unique index on `(project_id, equipment_id)`; index on
  `(owner_id, project_id)` and on `equipment_id` (for cascade delete).
  Attaching requires the caller to already own both the Project (checked
  against the existing SQLite-backed project ownership) and the Equipment.

All ownership checks happen server-side, keyed off the verified identity
`apps/api/internal/httpapi/auth.go`'s `ownerContext` middleware already
established (extended in this milestone to also carry the token's verified
`email`/`name` custom claims). A lookup for someone else's equipment returns
the same 404 as a nonexistent id — never a 403 that would confirm the id
belongs to someone else.

### API

```
GET    /api/v1/me
GET    /api/v1/catalog
GET    /api/v1/catalog/:id
POST   /api/v1/equipment
GET    /api/v1/equipment
GET    /api/v1/equipment/:id
PATCH  /api/v1/equipment/:id
DELETE /api/v1/equipment/:id
GET    /api/v1/projects/:id/equipment
POST   /api/v1/projects/:id/equipment
DELETE /api/v1/projects/:id/equipment/:equipmentId
```

Every one of these (except the two read-only catalog routes) requires a
verified, signed-in identity — unlike Projects, anonymous/Demo Mode callers
are rejected with `401 AUTHENTICATION_REQUIRED` rather than allowed to
create anonymous equipment.

## Tiger Data: telemetry (`apps/api/internal/telemetrystore`)

Answers "what happened electrically over time?" A Postgres/Timescale-
compatible `telemetry_measurements` table holds one flattened row per probe
per measurement window, mirroring firmware's own `TelemetrySample` fields
(state, edge counts, averaged analog/period/pulse-width values) rather than
inventing a new measurement vocabulary. Indexes on `(probe, time DESC)`,
`(profile_id, time DESC)`, `(device_id, time DESC)`, and `(time DESC)`.
`EnsureSchema` opportunistically calls `create_hypertable` — a no-op on
plain PostgreSQL, and exactly what turns the table into a real Timescale
hypertable on Tiger Data.

Every telemetry window that reaches the diagnostic engine is **dual-written**:
SQLite's existing `SaveMeasurement` runs first and remains authoritative; a
Tiger Data insert is attempted afterward as pure best-effort. A Tiger
failure (unconfigured, unreachable, or a write error) is logged and never
returned to the caller, and never means a measurement is lost — SQLite
already has it.

```
GET /api/v1/telemetry?probe=P1&profile_id=...&since_ms=...&until_ms=...&limit=100
```

`limit` is always clamped (max 1000, default 100) — this can never become an
unbounded scan. This is a different question, and a different response
shape, from the existing `GET /api/v1/measurements` (whole SQLite-backed
diagnostic windows); it does not replace or duplicate it.

Telemetry is never queried by PROBE/Gemini directly. The evidence Gemini
reasons about is always the already-computed `StructuredEvidence`/
`RuleResult`s the deterministic engine derived beforehand — this milestone
does not change that boundary.

## Environment variables

```
MONGODB_URI=
MONGODB_DATABASE=reweird
TIGER_DATABASE_URL=
```

See `.env.example` for the full, current list with inline documentation.

## External setup

- **MongoDB Atlas**: create a free-tier (or larger) cluster, a database user
  scoped to it, and a network access entry covering wherever the API runs.
  Set `MONGODB_URI` to that user's connection string and `MONGODB_DATABASE`
  to the database name (default `reweird`). Indexes are created
  automatically on startup (`EnsureIndexes`) — no manual Atlas index setup
  is required.
- **Tiger Data**: create a Tiger Data (or any Timescale-compatible Postgres)
  service and set `TIGER_DATABASE_URL` to its connection string. The table,
  indexes, and hypertable conversion are created automatically on startup
  (`EnsureSchema`) — no manual schema setup is required.

## Known limitations

- `GET /api/v1/me` (and therefore user persistence) only ever runs when a
  request actually calls it — a signed-in user who never opens a page that
  calls `/me` will not yet have a MongoDB user document, though `/equipment`
  itself still works from the verified subject alone.
- `project_equipment` scoping/cascade-delete queries are not paginated —
  fine at today's scale, but would need a limit/cursor if a project or
  owner's equipment list grows very large.
- Tiger Data query scoping does not join back to MongoDB Equipment/Project
  records; a telemetry response identifies probes/profiles/devices, not
  equipment names — callers already holding an Equipment/Project id can
  correlate them client-side.
