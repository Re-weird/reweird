## [Open the ReWeird demo →](https://re-weird.github.io/reweird/)

# ReWeird

![ReWeird logo](apps/web/public/images/reweird-logo.png)

Evidence-first diagnostics for physical electronics projects.

## Judges: open the interactive demo

No sign-in, API keys, installation, backend, or hardware needed. The public app
opens straight into our interactive simulation. All readings are explicitly
simulated in your browser; no data is sent to a server.

1. **Take a case.** Choose a mystery or practice a known fault.
2. **Investigate.** Inspect P1 (power), P2 (trigger), and P3 (echo). Run a test
   and follow the evidence. Hints are available.
3. **Name the cause.** A wrong answer lets you try again.
4. **Fix and verify.** Try a repair. ReWeird only declares the circuit fixed
   when all four checks pass.
5. **Download the record.** Save the simulated Device Passport as JSON, or try
   another case.

For a quick walkthrough: choose **Or practice a known fault**, pick the loose
ECHO connection, inspect P3, run the wiggle test, identify the loose connection,
and reseat the connector. Wrong tests and repairs also work: they leave the
circuit unresolved until you find the right fix.

The simulation covers four faults: a loose ECHO connection, unstable power,
a disconnected ECHO signal, and incorrect trigger timing. It demonstrates
**Detect → Diagnose → Test → Verify** using deterministic simulated evidence.
It does not claim to run live hardware measurements or an LLM in the browser.

## Run locally

Requires Node.js 20.9+ and npm. From this repository:

```bash
npm ci
npm run dev
```

Open [localhost:3000](http://localhost:3000). The root page, `/demo`, and `/try`
all open the simulation. There is no environment-file setup.

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
