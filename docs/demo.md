# Deterministic demo walkthrough

Start the API and web app as described in the README. Open **Settings & status** first: database should be `ok`, telemetry `simulator · available`, Gemini Vision may be `not configured`, ESP32 is not connected, Git sync is disabled, computer collection is not running, and PATCH is locked.

1. Click **Load demo** to select the separate HC-SR04 built-in project. View its confirmed Project Profile and probe placement information.
2. Open **Fault simulator**, select **Intermittent connection**, and run it. The API generates raw simulator samples, validates the telemetry contract, derives dropouts and stability, then stores a measurement window. The simulator name is not injected as a diagnostic fact.
3. Open **Guided test**, create the recommended movement-correlation plan, start, and capture the movement window. Observe the test result based on changed dropout behavior.
4. Simulate the correction and re-measure. Only the persisted before/after evidence can produce a resolved VERIFY status. If the after measurement does not improve, the workflow remains unresolved/inconclusive.
5. Open **History**, inspect the persisted timeline and profile revision, then **Reports** for measured vs. derived evidence, JSON/Markdown download, and print.
6. Click **Preview Git sync** on the report. With default settings, the proposed paths and secret-scan result appear, but commit is disabled. **Keep local** makes no Git changes.
7. Open **Computer diagnostics**, simulate **Port conflict**, and inspect the separate system evidence and non-destructive next action.

The demo uses simulated electrical samples, not physical hardware. The computer simulation is not a local-system scan. A real project never receives the HC-SR04 demo measurements when its confirmed profile does not match the active telemetry source. Gemini PROBE is optional and under separate development; it is not required for this walkthrough.
