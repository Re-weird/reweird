## [Open the ReWeird demo →](https://re-weird.github.io/reweird/)

# ReWeird

![ReWeird logo](apps/web/public/images/reweird-logo.png)

Evidence-first diagnostics for physical electronics projects.

## Judges: open the interactive demo

No sign-in, API keys, installation, backend, or hardware needed.
The original landing page gives judges two choices:

- **[Project demo](https://re-weird.github.io/reweird/software/)**: the actual
  ReWeird workbench and project views, using simulated values. Explore the
  circuit map, signal monitor, diagnosis, guided tests, VERIFY, Device Passport,
  history, and downloadable reports. Choose a fault and load new readings from
  the input selector. All state stays in the browser session.
- **[Try it yourself](https://re-weird.github.io/reweird/try/)**: the separate
  interactive circuit game. Find a fault, choose tests and repairs, and score
  your investigation.

The project demo reuses the actual app's Workbench, Project Overview, Circuit
Map, Probe Setup, Diagnosis, Simulator, and Guided Test components. Its browser
adapter supplies deterministic sample values instead of a hardware/API
connection. It does not call a live LLM or claim physical measurements.
The workflow is **Detect → Diagnose → Test → Verify**.

For a quick walkthrough, open **Project demo**, select **Loose connection**,
click **Load simulated values**, and choose **Run guided diagnosis**. Capture
the before and during windows, then **Simulate correction & VERIFY**. The
workbench and reports reflect the updated values. Try another fault or download
the report from the Reports tab.

## Run locally

Requires Node.js 20.9+ and npm. From this repository:

```bash
npm ci
npm run dev
```

Open [localhost:3000](http://localhost:3000). The root page shows the original landing design, `/demo` shows both choices,
`/software` opens the project demo, and `/try` opens the game. There is no environment-file setup.

For a production build:

```bash
npm run build
npm start
```

## Deployment and development

The **Deploy judge demo** GitHub Actions workflow publishes `main` to GitHub
Pages. The published files are static HTML, CSS, JavaScript, and public assets.
Only `*.judge.tsx` and `*.judge.ts` routes are included. Google sign-in, API
handlers, live project uploads, GitHub account connections, and backend proxies
are excluded from this build, even if old service credentials are configured.

```bash
npm run typecheck
npm run test:judge-demo --workspace @reweird/web
npm run build
```

The original hardware app, Go API, firmware, and service code remain available
for engineering work. They are not part of the hosted judge experience.
See [hardware architecture and reference](docs/hardware-reference.md) and
[deployment notes](docs/deployment.md). To deliberately run the original app,
set `REWEIRD_APP_MODE=hardware` before running the web dev/build command.
Do not enable that mode for the judge deployment.
