# Database architecture

ReWeird's diagnostic system is authoritative in SQLite (`apps/api/internal/store`)
and stays that way, including telemetry. This milestone adds one new,
purely additive persistence responsibility alongside it:

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
                                 Measurement windows (telemetry)
                                     ^
ESP32 / Simulator --- electrical telemetry
```

MongoDB is entirely optional. With `MONGODB_URI` unset, the API boots
exactly as it did before this milestone: SQLite-backed diagnostics, Demo
Mode, and the simulator all work unchanged. `GET /api/v1/me` and
`/api/v1/equipment*` degrade to a clear `503` instead of silently
no-op-ing or fabricating data.

Telemetry (timestamped electrical measurements) has no separate database.
`measurement_windows` in SQLite is, and remains, the single authoritative
telemetry store — an earlier version of this milestone added a second,
Tiger Data (Timescale) telemetry store; it was removed after review in
favor of extending the existing SQLite path instead (see "Telemetry" below).

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

## Telemetry: SQLite (`apps/api/internal/store`)

Answers "what happened electrically over time?" using the existing
`measurement_windows` table — one row per telemetry window (the full
firmware/simulator envelope plus its deterministic analysis result),
deduplicated on `(source, device_id, sequence, captured_at_ms)`.

`GET /api/v1/measurements` is the one measurement API (there is no second,
duplicate endpoint). It always supported `profile_id`/`limit`; this
milestone added further optional, bounded filters over the same table and
response shape:

```
GET /api/v1/measurements?profile_id=...&device_id=...&source=...&probe=P1&since_ms=...&until_ms=...&limit=50
```

- `profile_id`, `device_id`, `source`, `since_ms`/`until_ms` are indexed SQL
  `WHERE` filters (see indexes below).
- `probe` is a best-effort filter: `measurement_windows` stores whole
  windows (many probes per row), not one row per probe, so there is no
  probe column to index. A `probe` filter overfetches a bounded batch
  (indexed by whatever other filters are given, capped regardless) and
  filters in Go, then truncates to the requested `limit`.
- `limit` is always clamped to `[1, 200]`, exactly as before.

Indexes on `measurement_windows`: the pre-existing `(profile_id,
captured_at_ms DESC, id DESC)` and `(profile_id, ingested_at_ms DESC, id
DESC)`, plus three added this milestone: `(device_id, captured_at_ms DESC,
id DESC)`, `(source, captured_at_ms DESC, id DESC)`, and `(captured_at_ms
DESC, id DESC)` for time-range/latest queries with no profile filter.

Telemetry is never queried by PROBE/Gemini directly. The evidence Gemini
reasons about is always the already-computed `StructuredEvidence`/
`RuleResult`s the deterministic engine derived beforehand from these
measurements — this milestone does not change that boundary.

## Environment variables

```
MONGODB_URI=
MONGODB_DATABASE=reweird
```

See `.env.example` for the full, current list with inline documentation.

## External setup

- **MongoDB Atlas**: create a free-tier (or larger) cluster, a database user
  scoped to it, and a network access entry covering wherever the API runs.
  Set `MONGODB_URI` to that user's connection string and `MONGODB_DATABASE`
  to the database name (default `reweird`). Indexes are created
  automatically on startup (`EnsureIndexes`) — no manual Atlas index setup
  is required.
- Telemetry requires no external setup at all — it is SQLite, exactly as
  it always was.

## Known limitations

- `GET /api/v1/me` (and therefore user persistence) only ever runs when a
  request actually calls it — a signed-in user who never opens a page that
  calls `/me` will not yet have a MongoDB user document, though `/equipment`
  itself still works from the verified subject alone.
- `project_equipment` scoping/cascade-delete queries are not paginated —
  fine at today's scale, but would need a limit/cursor if a project or
  owner's equipment list grows very large.
- `probe`-scoped measurement queries are bounded, best-effort overfetch-
  then-filter (see above), not a genuinely indexed lookup — fine at
  today's scale; a per-probe index or table would be needed if this ever
  became a bottleneck at much larger history sizes.
