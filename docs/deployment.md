# Deployment

## Web (Vercel)

The Next.js app (`apps/web`) is hosted on Vercel as project `web`
(org `evinbento09-3005s-projects`), with **Root Directory** set to
`apps/web` so it can resolve the workspace-local `@reweird/shared-types`
package.

Vercel's own GitHub integration is **not** connected to this repo (it
failed to auto-link during setup). Instead, `.github/workflows/deploy-web.yml`
drives the same `vercel` CLI commands the integration would run, gated on
`.github/workflows/ci.yml` passing on `main` first.

### Required repo secrets

Set these under **Settings → Secrets and variables → Actions**:

| Secret | Where to get it |
|---|---|
| `VERCEL_TOKEN` | [vercel.com/account/tokens](https://vercel.com/account/tokens) — create one scoped to this project if possible. |
| `VERCEL_ORG_ID` | `team_PpqYquP1GLCMYCZZrtweVmCB` (this org). Also in `.vercel/project.json` after running `vercel link` locally. |
| `VERCEL_PROJECT_ID` | `prj_H3RGnzFRv1dv79VsXfQX7qoxJ4tU` (the `web` project). Same file. |

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

## API (Go) — not yet deployed

Planned host: Railway. Not set up yet. Once it is:

1. Add the Railway URL as `API_INTERNAL_URL` in Vercel's env vars (all
   targets that should reach it) and redeploy the web app.
2. Point the GitHub App's webhook URL (`docs/github-app.md`) at the Railway
   URL's `/webhooks/github`.
3. Add a deploy job here for the API once Railway's deploy method (CLI,
   GitHub integration, or Docker registry push) is decided.
