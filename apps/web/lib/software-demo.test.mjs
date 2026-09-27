import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const urls = new Map();
function compile(name) {
  if (urls.has(name)) return urls.get(name);
  let code = ts.transpileModule(readFileSync(new URL(`./${name}.ts`, import.meta.url), "utf8"), { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
  code = code.replace(/from "\.\/([^"\n]+)"/g, (_, dep) => `from "${compile(dep)}"`);
  const url = `data:text/javascript;base64,${Buffer.from(code).toString("base64")}`;
  urls.set(name, url); return url;
}
const demo = await import(compile("software-demo"));
test("all software scenarios supply real-view contracts without claiming physical hardware", () => {
  for (const scenario of demo.scenarios) {
    const session = demo.softwareSession(demo.snapshotFor(scenario.id), scenario.id);
    assert.equal(session.hardware_connected, false);
    assert.equal(session.telemetry_mode, "browser");
    assert.equal(session.raw_telemetry.profile_id, demo.softwareProfile().id);
    assert.equal(session.probes.length, 3);
    assert.equal(session.analysis.probes.length, 3);
    assert.equal(session.evidence.rule_results.every(rule => rule.status === "pass"), scenario.id === "healthy");
  }
});
test("guided scenarios produce before/during captures and healthy after evidence", () => {
  for (const scenario of demo.scenarios.filter(s => s.id !== "healthy")) {
    const plan = demo.planWorkflow(scenario.id);
    const during = demo.testSnapshot(scenario.id);
    const session = demo.softwareSession(during.snapshot, scenario.id, "test", 2);
    assert.equal(plan.plan.requires_patch, false);
    assert.ok(during.finding.length > 10);
    assert.equal(demo.measurement(session).source, "SIMULATED");
    assert.equal(demo.measurement(session).id, 2);
    const after = demo.softwareSession(demo.snapshotFor("healthy"), scenario.id, "verify", 3);
    assert.ok(after.evidence.rule_results.every(rule => rule.status === "pass"));
    assert.equal(after.after.dropouts_per_minute, 0);
  }
});
test("wiggle changes ECHO evidence and keeps supply and trigger stable", () => {
  const before = demo.snapshotFor("loose-echo");
  const during = demo.testSnapshot("loose-echo").snapshot;
  assert.ok(during.echo.dropouts_per_min > before.echo.dropouts_per_min);
  assert.deepEqual(during.power, before.power);
  assert.deepEqual(during.trig, before.trig);
});
