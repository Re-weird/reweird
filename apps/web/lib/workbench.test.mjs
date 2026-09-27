import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";
import ts from "typescript";

// Test the actual rendered timeline, not a duplicate tick implementation.
const require = createRequire(import.meta.url);
const source = readFileSync(new URL("../app/workbench.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("workbench.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const timeline = ast.statements.find((node) => ts.isFunctionDeclaration(node) && node.name?.text === "ActivityTimeline");
assert.ok(timeline);
const compiled = ts.transpileModule(`export ${timeline.getText(ast)}`, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 },
}).outputText;
const exports = {};
vm.runInNewContext(compiled, { exports, require, useState: () => [null, () => {}], cn: (...args) => args.filter(Boolean).join(" ") });
function render(windowMS, probes = [{ probe: "P2", role: "TRIG", unit: "pulses/s", samples: [0, 0, 1, 0, 0, 1, 0, 0, 1, 0] }]) {
  return exports.ActivityTimeline({ probes, windowMS });
}
function walk(node, visit) {
  if (!node || typeof node !== "object") return;
  if (Array.isArray(node)) {
    const keys = node.filter((child) => child && typeof child === "object" && child.key != null).map((child) => child.key);
    assert.equal(new Set(keys).size, keys.length, "Sibling keys must be unique even when measured/displayed values repeat");
    node.forEach((child) => walk(child, visit));
  } else { visit(node); walk(node.props?.children, visit); }
}
function ticks(tree) {
  const result = [];
  walk(tree, (node) => { if (node.type === "span" && Array.isArray(node.props.children) && node.props.children[1] === "s") result.push(node); });
  return result;
}
test("one-second capture preserves repeated 0/1 labels with unique stable tick keys", () => {
  const first = ticks(render(1000));
  assert.deepEqual(first.map((tick) => tick.props.children[0]), [0, 0, 1, 1, 1]);
  assert.equal(first.length, 5);
  assert.deepEqual(ticks(render(1000)).map((tick) => tick.key), first.map((tick) => tick.key));
  assert.deepEqual(ticks(render(60000)).map((tick) => tick.key), first.map((tick) => tick.key));
});
test("all-zero ticks and repeated activity values remain valid; absent samples render nothing", () => {
  assert.equal(ticks(render(100)).length, 5);
  assert.equal(render(1000, []), null);
  assert.equal(render(1000, [{ probe: "P2", samples: null }]), null);
});

test("restored signal panel uses sparkline cards, area chart and rule rows with current evidence", () => {
  const node = ast.statements.find((n) => ts.isFunctionDeclaration(n) && n.name?.text === "SignalMonitor");
  const output = ts.transpileModule(`export ${node.getText(ast)}`, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
  const scope = { exports: {}, require, ProbeCard: "ProbeCard", RuleRow: "RuleRow", SignalChart: "SignalChart", Bolt: "Bolt", Radio: "Radio", ChevronRight: "ChevronRight" };
  vm.runInNewContext(output, scope);
  const session = { probes: [{ probe: "P2", status: "active", samples: [0, 1, 0] }], raw_telemetry: { window_ms: 1000 }, evidence: { rule_results: [{ id: "loss", probe: "P2", status: "fail", message: "Real captured evidence" }] } };
  const tree = scope.exports.SignalMonitor({ session, live: true, plan: null, failures: 1, onNavigate() {} });
  const elements = [];
  walk(tree, (n) => elements.push(n));
  assert.equal(tree.props.className, "bench-panel bench-signals");
  assert.equal(elements.find((n) => n.type === "ProbeCard").props.reading, session.probes[0]);
  assert.equal(elements.find((n) => n.type === "RuleRow").props.rule, session.evidence.rule_results[0]);
  assert.equal(elements.find((n) => n.type === "SignalChart").props.session, session);
  assert.equal(elements.find((n) => n.type === "SignalChart").props.windowMS, 1000);
  assert.ok(elements.some((n) => n.props.className === "two-column wide-left"));
});

test("restored area chart labels the real one-second window without changing sample values", () => {
  const text = readFileSync(new URL("../app/signal-components.tsx", import.meta.url), "utf8");
  const file = ts.createSourceFile("signal-components.tsx", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const node = file.statements.find((n) => ts.isFunctionDeclaration(n) && n.name?.text === "SignalChart");
  const output = ts.transpileModule(node.getText(file), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } }).outputText;
  const scope = { exports: {}, require, ResponsiveContainer: "ResponsiveContainer", AreaChart: "AreaChart", CartesianGrid: "CartesianGrid", XAxis: "XAxis", YAxis: "YAxis", Tooltip: "Tooltip", Area: "Area" };
  vm.runInNewContext(output, scope);
  const samples = [0, 0, 1, 0, 0, 1, 0, 0, 1, 0];
  const session = { probes: [{ probe: "P2", role: "TRIG", samples }] };
  const tree = scope.exports.SignalChart({ session, windowMS: 1000 });
  const elements = []; walk(tree, (n) => elements.push(n));
  const rows = elements.find((n) => n.type === "AreaChart").props.data;
  assert.deepEqual(Array.from(rows, (r) => r.primary), samples);
  assert.equal(rows[0].time, "0s"); assert.equal(rows[9].time, "0.9s");
  assert.equal(new Set(rows.map((r) => r.time)).size, 10);
});
