import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./weird-demo.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const { availableWeirdChoices, mysteryScenario, realBreakReady, telemetryLabel, testHeadline, verifyHeadline } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

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

test("physical break state requires matching real capture and confirmed setup", () => {
  const session = { hardware_connected: true, telemetry_mode: "serial", profile_id: "p", raw_telemetry: { profile_id: "p" }, measurement_id: 8 };
  const profile = { id: "p", confirmed: true };
  const plan = { profile_id: "p", connected: true };
  assert.equal(realBreakReady(session, "api", profile, plan), true);
  assert.equal(realBreakReady({ ...session, telemetry_mode: "simulator" }, "api", profile, plan), false);
  assert.equal(realBreakReady(session, "browser", profile, plan), false);
  assert.equal(realBreakReady(session, "api", profile, { ...plan, connected: false }), false);
  assert.equal(realBreakReady({ ...session, raw_telemetry: { profile_id: "other" } }, "api", profile, plan), false);
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
