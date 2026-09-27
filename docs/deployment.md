# Deployment

## Web (Vercel)

The Next.js app (`apps/web`) is hosted on Vercel as project `web`
(org `evinbento09-3005s-projects`), with **Root Directory** set to
`apps/web` so it can resolve the workspace-local `@reweird/shared-types`
package.

Vercel's own GitHub integration is **not** connected to this repo (it
failed to auto-link during setup). A GitHub Actions-driven auto-deploy
(`deploy-web.yml`, mirroring `deploy-api.yml`) was tried and dropped: every
`VERCEL_TOKEN` created for it failed Vercel's own `whoami`/`v2/user` lookup
("User not found") regardless of how it was scoped, which pointed at an
account-side issue with that token rather than anything in this repo.
Production is deployed manually instead (see below) until that's resolved.

### Environment variables (on Vercel, not GitHub)

Set for Production, Preview, and Development in the Vercel project itself
(`vercel env add <NAME> <target>`), not as GitHub secrets — the deploy
workflow doesn't pass these through, Vercel injects them at build/runtime:

- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` — Google OAuth. The redirect URI
  `https://<domain>/api/auth/callback/google` must be added to the OAuth
  client's Authorized redirect URIs for every domain that needs to sign in
  (production domain, and each preview domain if you test sign-in there).
- `NEXT_PUBLIC_GOOGLE_CLIENT_ID` — same value as `GOOGLE_CLIENT_ID`, exposed
  to the browser. Must be added with `--type config`, not `--sensitive`;
  Vercel rejects `NEXT_PUBLIC_*` as a Secret since it's not actually private.
- `AUTH_SECRET`, `AUTH_TOKEN_SECRET` — random values, used to sign sessions
  and the token `apps/api` verifies for ownership scoping.
- `API_INTERNAL_URL` — the Go API's public URL once it's deployed (Railway,
  planned). Until then, `/api/*` and `/webhooks/github` rewrites in
  `next.config.ts` fall back to `http://127.0.0.1:8080`, which is
  unreachable from Vercel, so those calls fail. Frontend pages that don't
  need the API still work.

`trustHost: true` is set directly in `apps/web/auth.ts`. On Vercel this is
enough by itself — Vercel's own infra correctly forwards the real host, so
no `AUTH_URL` override is needed (that was only required for the earlier
ngrok-based local setup, which had no way to reconstruct the real origin
from forwarded headers).

### Manual deploy

```bash
cd reweird  # repo root; Root Directory is apps/web, but link/build run from here
vercel link --yes   # first time only, or if .vercel/project.json is missing
vercel --prod --yes
```

`.vercelignore` at the repo root excludes `node_modules`, `.git`, `.next`,
`apps/api/reweird.db` (the local SQLite file — multiple hundred MB, never
needs to leave this machine), `apps/api/data`, and the PlatformIO build
cache. Keep it in sync if new large/generated paths show up.

## API (Go) — Railway

The Go API (`apps/api`) is hosted on Railway as service `reweird-api` in
project `ReWierd` (workspace "Evin Bento's Projects"), built from
`apps/api/Dockerfile` with the repo root as build context — `railway.json`
at the repo root tells Railway's builder to use that Dockerfile instead of
auto-detecting a builder. `.railwayignore` excludes `node_modules`, `.git`,
`.next`, and the local dev sqlite db from the build context.

Railway assigns a dynamic `$PORT`; `apps/api/cmd/server/main.go` reads
`PORT`, falling back to `API_PORT`, then `8080`, so it binds whatever
Railway expects.

A Railway volume is mounted at `/data`. `DATABASE_PATH=/data/reweird.db` and
`UPLOAD_DIR=/data/uploads` point the app at it so the sqlite db and uploaded
files survive redeploys. The Dockerfile runs as a non-root user; a small
`docker-entrypoint.sh` chowns those paths under `/data` at container start
(Railway mounts volumes owned by root, unwritable by the app's user,
regardless of the image's `USER`) before dropping privileges.

Public URL: `https://reweird-api-production.up.railway.app`.

### Required env vars (on Railway, not GitHub)

Set via `railway variable set <NAME> --service reweird-api`:

- `API_HOST=0.0.0.0`, `API_TRUSTED_NETWORK=true` — the app refuses to bind a
  non-loopback host without either `REWEIRD_API_TOKEN` (a static bearer
  token gate) or this flag. `REWEIRD_API_TOKEN` isn't used here because the
  same `Authorization: Bearer` header already carries each user's session
  JWT (verified against `AUTH_TOKEN_SECRET`, see `apps/api/internal/httpapi/auth.go`)
  — a static token would collide with that. `API_TRUSTED_NETWORK=true`
  means this deployment relies on that per-user JWT check instead; requests
  with no token are treated as anonymous/Demo Mode, same as running on a
  trusted LAN.
- `AUTH_TOKEN_SECRET` — must match the value set on Vercel, so tokens
  `apps/web`'s `/api/auth/token` route mints verify here.
- `TELEMETRY_MODE=simulator` — no serial hardware is attached to Railway.
- `DATABASE_PATH=/data/reweird.db`, `UPLOAD_DIR=/data/uploads` — see above.
- `PROJECT_PROFILE_ID=ultrasonic-demo` — the seeded demo profile.
- `GITHUB_APP_ID`, `GITHUB_APP_SLUG`, `GITHUB_APP_CLIENT_ID`,
  `GITHUB_APP_CLIENT_SECRET`, `GITHUB_WEBHOOK_SECRET` — same values as local
  `.env`.
- `GITHUB_APP_PRIVATE_KEY` — the PEM contents directly (not
  `GITHUB_APP_PRIVATE_KEY_PATH`; there's no local file to point at on
  Railway).
- `GEMINI_API_KEY` — optional, not currently set; vision analysis stays
  disabled without it, same as local dev without the key.

### Required repo secrets (for `.github/workflows/deploy-api.yml`)

| Secret | Where to get it |
|---|---|
| `RAILWAY_TOKEN` | Railway dashboard → project `ReWierd` → Settings → Tokens. Scope it to this project if possible. |

Railway's own GitHub integration (connected via `railway service source connect`)
also redeploys on every push to `main`, independent of CI. The GitHub Actions
workflow adds an explicit CI-gated redeploy on top of that, matching the web
deploy's pattern.

### Remaining steps

1. Add `API_INTERNAL_URL=https://reweird-api-production.up.railway.app` to
   Vercel's env vars (all targets) and redeploy the web app.
2. Point the GitHub App's webhook URL (`docs/github-app.md`) at
   `https://reweird-api-production.up.railway.app/webhooks/github`.
