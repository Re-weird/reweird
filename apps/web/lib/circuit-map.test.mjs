import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./circuit-map.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const { buildCircuitMap, circuitDisplayState } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

test("trace animation uses longhands without resetting its stagger delay", () => {
  const component = readFileSync(new URL("../app/circuit-map.tsx", import.meta.url), "utf8");
  const file = ts.createSourceFile("circuit-map.tsx", component, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  let style;
  function visit(node) {
    if (ts.isObjectLiteralExpression(node) && node.properties.some((p) => p.name?.getText(file) === "animationDelay")) style = node;
    ts.forEachChild(node, visit);
  }
  visit(file);
  assert.ok(style);
  assert.ok(!style.properties.some((p) => p.name?.getText(file) === "animation"));
  const js = ts.transpileModule(`const style = ${style.getText(file)};`, { compilerOptions: { target: ts.ScriptTarget.ES2020 } }).outputText;
  for (const state of ["suspect", "healthy"]) {
    const value = new Function("trace", "index", `${js}; return style;`)({ state }, 2);
    assert.deepEqual(value, { animationName: "trace-run", animationDuration: state === "suspect" ? "3.4s" : "2.2s", animationTimingFunction: "linear", animationIterationCount: "infinite", animationDelay: "0.7s" });
  }
});

function profile() {
  return {
    id: "profile-a", project_id: "project-a", confirmed: true,
    components: [{ id: "motor", name: "Motor", confirmed: true }],
    connections: [
      { id: "speed", component_id: "motor", component_name: "Motor", role: "PWM", target: "GPIO7 / Motor PWM", gpio: 7, expected: { signal_type: "PWM" }, sources: ["USER"] },
      { id: "power", component_id: "motor", component_name: "Motor", role: "VCC", target: "Motor 5 V", expected: { signal_type: "voltage" }, sources: ["USER"] },
    ],
  };
}

function plan() {
  return {
    project_id: "project-a", profile_id: "profile-a", connected: true,
    instructions: [
      { probe: "GND", role: "reference", target: "Ground" },
      { probe: "P1", role: "PWM", target: "GPIO7 / Motor PWM" },
      { probe: "P2", role: "VCC", target: "Motor 5 V" },
    ],
  };
}

function capture() {
  return {
    profile_id: "profile-a", measurement_id: 22, telemetry_mode: "serial",
    raw_telemetry: { profile_id: "profile-a" }, analysis: { profile_id: "profile-a" },
    probes: [{ probe: "P1", status: "active", value: 50, unit: "Hz" }, { probe: "P2", status: "stable", value: 5.01, unit: "V" }],
    evidence: { rule_results: [{ id: "pwm", probe: "P1", status: "pass" }] },
  };
}

test("maps confirmed connections and exact probe instructions without fixed pin assumptions", () => {
  const result = buildCircuitMap(profile(), plan(), capture());
  assert.equal(result.groups[0].connections[0].connection.gpio, 7);
  assert.equal(result.groups[0].connections[0].instruction.probe, "P1");
  assert.equal(result.groups[0].connections[1].instruction.probe, "P2");
  assert.equal(result.groups[0].connections[0].reading.value, 50);
  assert.equal(result.groups[0].connections[0].rules[0].id, "pwm");
  assert.equal(result.captureKind, "serial");
});

test("does not overlay demo or mismatched telemetry onto another project", () => {
  for (const altered of [
    { ...capture(), profile_id: "ultrasonic-demo" },
    { ...capture(), raw_telemetry: { profile_id: "other" } },
    { ...capture(), analysis: { profile_id: "other" } },
    { ...capture(), measurement_id: undefined },
    { ...capture(), measurement_id: 0 },
  ]) {
    const result = buildCircuitMap(profile(), plan(), altered);
    assert.equal(result.hasMatchingCapture, false);
    assert.equal(result.groups[0].connections[0].reading, undefined);
  }
});

test("drafts and unconnected plans show model only", () => {
  assert.equal(buildCircuitMap({ ...profile(), confirmed: false }, plan(), capture()).groups[0].connections[0].instruction, undefined);
  assert.equal(buildCircuitMap(profile(), { ...plan(), connected: false }, capture()).hasMatchingCapture, false);
  assert.equal(buildCircuitMap(profile(), { ...plan(), profile_id: "other" }, capture()).groups[0].connections[0].instruction, undefined);
});

test("built-in demo may show its own stored simulator capture without a connection claim", () => {
  const result = buildCircuitMap(profile(), { ...plan(), connected: false }, { ...capture(), telemetry_mode: "simulator" }, true);
  assert.equal(result.hasMatchingCapture, true);
  assert.equal(result.captureKind, "simulator");
});

test("preserves uninstrumented connections and components without inventing a probe", () => {
  const next = profile();
  next.components.push({ id: "led", name: "LED", confirmed: true });
  next.connections.push({ id: "ground", component_id: "motor", component_name: "Motor", role: "GND", target: "Motor ground", expected: { signal_type: "ground" }, sources: ["USER"] });
  const result = buildCircuitMap(next, plan(), capture());
  assert.equal(result.groups[0].connections[2].instruction, undefined);
  assert.equal(result.groups[1].name, "LED");
  assert.equal(result.groups[1].connections.length, 0);
});

test("fault, testing, and recovery highlights require matching captured evidence", () => {
  const bad = { ...capture(), probes: [{ probe: "P1", status: "intermittent", value: 0, unit: "Hz" }], evidence: { rule_results: [{ id: "pwm", probe: "P1", status: "fail" }] } };
  const mapped = buildCircuitMap(profile(), plan(), bad);
  const item = mapped.groups[0].connections[0];
  assert.equal(circuitDisplayState(item, mapped.hasMatchingCapture, bad, null), "suspect");
  assert.equal(circuitDisplayState(item, false, bad, null), "waiting");
  const workflow = { profile_id: "profile-a", status: "WAITING_FOR_USER", plan: { recommendation: { target_probes: ["P1"] } } };
  assert.equal(circuitDisplayState(item, true, bad, workflow), "testing");
  const resolved = { ...workflow, status: "RESOLVED", verification: { status: "RESOLVED", after_window_id: 22, changes: [{ probe: "P1" }] } };
  const good = buildCircuitMap(profile(), plan(), capture()).groups[0].connections[0];
  assert.equal(circuitDisplayState(good, true, capture(), resolved), "recovered");
  assert.equal(circuitDisplayState(item, true, bad, resolved), "suspect");
  assert.equal(circuitDisplayState(item, true, { ...bad, measurement_id: 23 }, resolved), "suspect");
  assert.equal(circuitDisplayState(item, true, bad, { ...resolved, profile_id: "other" }), "suspect");
});
