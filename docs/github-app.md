# GitHub App setup

ReWeird reads each project's firmware from a GitHub repository. Users connect
GitHub once in **Settings**, then pick a repository when they create a
project. Every push to that repository's default branch is read and analyzed
again.

ReWeird connects as a **GitHub App**, not an OAuth App:

- Users choose exactly which repositories it can see.
- Access is read-only.
- Tokens are short-lived installation tokens minted from the App's private key.
- ReWeird never stores a GitHub user token.

## 1. Register the App

On GitHub, go to **Settings → Developer settings → GitHub Apps → New GitHub
App** (or the same page under an organization).

| Field | Value (local development) |
|---|---|
| GitHub App name | anything unique, e.g. `reweird-dev-<you>` |
| Homepage URL | `http://localhost:3000` |
| Callback URL | `http://localhost:3000/settings/github/callback` |
| Request user authorization (OAuth) during installation | **checked** (required: ReWeird uses this to confirm who installed the App) |
| Setup URL | leave empty (the callback above is used) |
| Redirect on update | **checked** |
| Webhook → Active | checked if GitHub can reach your API (see step 3); otherwise unchecked |
| Webhook URL | `https://<public-host>/webhooks/github` |
| Webhook secret | a long random string, e.g. `openssl rand -hex 32` |
| Repository permissions → Contents | **Read-only** |
| Repository permissions → Metadata | Read-only (set automatically) |
| Subscribe to events | **Push** |
| Where can this GitHub App be installed? | Only on this account (dev), or Any account |

After creating it:

- Note the **App ID** and **Client ID**.
- Generate a **client secret**.
- Generate a **private key**. This downloads a `.pem` file; keep it outside the repo.
- Note the slug: the last part of the App's public URL, `github.com/apps/<slug>`.

## 2. Configure the API

Add these to the root `.env`. The API reads them at startup.

```
GITHUB_APP_ID=123456
GITHUB_APP_SLUG=reweird-dev-you
GITHUB_APP_CLIENT_ID=Iv23li...
GITHUB_APP_CLIENT_SECRET=...
GITHUB_APP_PRIVATE_KEY_PATH=/absolute/path/to/reweird-dev.private-key.pem
GITHUB_WEBHOOK_SECRET=...
GITHUB_POLL_SECONDS=60
```

Fill in all of them or none. A partial configuration stops the API at
startup with a list of what's missing. Start the API with the file loaded:

```
cd apps/api
set -a; . ../../.env; set +a
go run ./cmd/server
```

The startup log says `GitHub App: configured`.

## 3. Receiving pushes

- **Deployed:** point the webhook at `https://<api-host>/webhooks/github`. It
  sits outside `/api/v1` because GitHub can't send the API bearer token. Every
  delivery is checked against `GITHUB_WEBHOOK_SECRET` (`X-Hub-Signature-256`)
  and rejected if the signature doesn't match.
- **Localhost:** GitHub can't reach your machine. Either:
  - rely on the poller, which checks each linked repo's default branch every
    `GITHUB_POLL_SECONDS` (default 60); or
  - relay webhooks with https://smee.io:
    `npx smee-client --url https://smee.io/<channel> --target http://127.0.0.1:8080/webhooks/github`,
    using the smee URL as the App's webhook URL.
- **Any time:** the project Overview has **Check for new commits**.

## What gets analyzed

On each new commit to the default branch, ReWeird:

1. Lists the files at that commit.
2. Keeps `.ino`, `.cpp`, `.c`, `.h`, `.hpp` or `.py` files. The analyzer
   parses one language, so it keeps whichever family, C/C++ or Python, has
   more code.
3. Skips:
   - dependency and build folders (`.pio`, `node_modules`, `build`, `vendor`, …)
   - files whose names look like credentials (`arduino_secrets.h`, `.env`, anything with `secret`, `token` or `password`)
   - files over 128 KB
4. Sends up to 512 KB of source in total to the static code analyzer.

Code is read as text and is never compiled or run. The Overview shows which
commit was analyzed and how many files were included or skipped.

Once a project's profile is **confirmed**, it doesn't change. A later push is
recorded as "New commit not analyzed" and does not replace it.

## Security notes

- A connection is tied to the signed-in ReWeird user. The install round trip
  carries a signed, expiring `state`. The API then exchanges GitHub's code for
  a user token only to confirm that user can access the installation, and
  discards the token.
- A project can only link a repository that the creating user's own
  installation can see.
- Uninstalling the App on GitHub sends an `installation.deleted` event that
  removes the connection. Disconnecting in Settings forgets the installation on
  ReWeird's side; the App stays installed on GitHub until it's removed there.
