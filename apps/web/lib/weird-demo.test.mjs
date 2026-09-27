import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./weird-demo.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const { availableWeirdChoices, mysteryScenario, realBreakReady, telemetryLabel, testHeadline, verifyHeadline, isCompatibleSerialSession, sessionForView, serialWorkflowMatches, serialRecommendationMatches } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

const scenarios = ["intermittent-connection", "unstable-power", "dead-signal", "timing-drift"].map((id) => ({ id }));

test("judge choices use only advertised existing simulator scenarios", () => {
  assert.deepEqual(availableWeirdChoices(scenarios).map((choice) => choice.scenario), scenarios.map((scenario) => scenario.id));
  assert.deepEqual(availableWeirdChoices([{ id: "healthy" }]), []);
  assert.equal(mysteryScenario(scenarios, () => 0), scenarios[0].id);
  assert.equal(mysteryScenario(scenarios, () => .999), scenarios[3].id);
  assert.equal(mysteryScenario([{ id: "healthy" }]), null);
});

test("source label distinguishes browser, API simulator, and serial", () => {
  assert.equal(telemetryLabel({ telemetry_mode: "serial" }, "browser"), "BROWSER SIMULATOR");
  assert.equal(telemetryLabel({ telemetry_mode: "simulator" }, "api"), "API SIMULATOR");
  assert.equal(telemetryLabel({ telemetry_mode: "serial" }, "api"), "REAL SERIAL");
});

test("connected serial status with rejected P5 mode never falls back to browser or cached simulator analysis", () => {
  const status = { mode: "serial", connected: true, device_id: "reweird-3428B5AD4F7C", profile_id: "ultrasonic-demo" };
  const cachedSimulator = { telemetry_mode: "simulator", profile_id: "ultrasonic-demo", measurement_id: 1014, raw_telemetry: { device_id: "reweird-simulator-001", profile_id: "ultrasonic-demo" } };
  assert.equal(isCompatibleSerialSession(status, null), false, "the API returned 503 after P5 analog failed its digital profile check");
  assert.equal(isCompatibleSerialSession(status, cachedSimulator), false, "a stored simulator window is not the active serial capture");
  assert.equal(sessionForView(false, null, cachedSimulator), null, "Workbench and Diagnosis must show unavailable, not 5.01 V / 40 kHz fixture data");
  assert.equal(sessionForView(true, null, cachedSimulator), cachedSimulator, "simulation is visible only in Practice simulator");
  const serial = { telemetry_mode: "serial", profile_id: "ultrasonic-demo", measurement_id: 1015, raw_telemetry: { device_id: status.device_id, profile_id: status.profile_id } };
  assert.equal(isCompatibleSerialSession(status, serial), true);
  assert.equal(sessionForView(false, serial, cachedSimulator), serial, "returning from Practice restores REAL SERIAL rather than cached simulation");
  assert.equal(isCompatibleSerialSession(status, { ...serial, raw_telemetry: { ...serial.raw_telemetry, profile_id: "other" } }), false);
});

test("a serial test planner cannot resume a simulator workflow for the same demo profile", () => {
  const serial = { id: "session-ultrasonic-demo", telemetry_mode: "serial", profile_id: "ultrasonic-demo", raw_telemetry: { device_id: "reweird-3428B5AD4F7C" } };
  assert.equal(serialWorkflowMatches({ profile_id: serial.profile_id, scenario_id: "intermittent-connection" }, serial), false);
  assert.equal(serialWorkflowMatches({ profile_id: serial.profile_id, baseline: { source: "simulator", device_id: "reweird-simulator-001" } }, serial), false);
  assert.equal(serialWorkflowMatches({ profile_id: serial.profile_id, baseline: { source: "serial", device_id: serial.raw_telemetry.device_id } }, serial), true);
  assert.equal(serialRecommendationMatches({ session_id: serial.id }, serial), true);
  assert.equal(serialRecommendationMatches({ session_id: "old-browser-demo" }, serial), false);
});

test("physical break state requires matching real capture and confirmed setup", () => {
  const session = { hardware_connected: true, telemetry_mode: "serial", profile_id: "p", raw_telemetry: { profile_id: "p" }, measurement_id: 8 };
  const profile = { id: "p", confirmed: true, probes: [{ role: "ECHO" }] };
  const plan = { profile_id: "p", connected: true };
  assert.equal(realBreakReady(session, "api", profile, plan), true);
  assert.equal(realBreakReady({ ...session, telemetry_mode: "simulator" }, "api", profile, plan), false);
  assert.equal(realBreakReady(session, "browser", profile, plan), false);
  assert.equal(realBreakReady(session, "api", profile, { ...plan, connected: false }), false);
  assert.equal(realBreakReady({ ...session, raw_telemetry: { profile_id: "other" } }, "api", profile, plan), false);
  assert.equal(realBreakReady(session, "api", { ...profile, probes: [{ role: "POWER" }] }, plan), false);
});

test("success copy is gated on resolved deterministic VERIFY", () => {
  assert.equal(verifyHeadline({ status: "RESOLVED", verification: { status: "RESOLVED" } }), "NOT WEIRD ANYMORE.");
  assert.equal(verifyHeadline({ status: "UNRESOLVED", verification: { status: "RESOLVED" } }), "STILL WEIRD.");
  assert.equal(verifyHeadline({ status: "UNRESOLVED", verification: { status: "IMPROVED" } }), "STILL WEIRD.");
  assert.equal(verifyHeadline({ status: "READY" }), null);
});

test("test copy only asserts support for positive results", () => {
  assert.equal(testHeadline({ result: { result: "POSITIVE_CORRELATION" } }), "WE FOUND SOMETHING.");
  assert.equal(testHeadline({ result: { result: "NO_CORRELATION_OBSERVED" } }), "THAT WASN’T IT.");
  assert.equal(testHeadline({ result: { result: "INCONCLUSIVE" } }), "STILL WEIRD.");
});
