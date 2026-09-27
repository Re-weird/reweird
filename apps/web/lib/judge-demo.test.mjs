import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./judge-demo.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const demo = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
const { reduce, canGoBack, hint, latestCapture, initialState, checks, verifies, current, revealed, found, verifyHeadline, incidentRecord, pickMystery, BASELINE, FAULTS, FAULT_ORDER, TEST_ORDER, FIX_ORDER } = demo;

const run = (...actions) => actions.reduce(reduce, initialState);
const started = (fault) => run({ type: "start", now: 1 }, { type: "make-weird", fault });

test("the healthy baseline passes every check and verifies", () => {
  assert.deepEqual(checks(BASELINE).map((c) => c.status), ["pass", "pass", "pass", "pass"]);
  assert.equal(verifies(BASELINE), true);
});

test("every fault fails at least one deterministic check and does not verify", () => {
  for (const id of FAULT_ORDER) {
    const symptom = FAULTS[id].symptom;
    assert.ok(checks(symptom).some((c) => c.status === "fail"), id);
    assert.equal(verifies(symptom), false, id);
  }
});

test("exactly one test finds each fault; the others say it wasn't it", () => {
  for (const id of FAULT_ORDER) {
    let state = started(id);
    for (const t of TEST_ORDER) state = reduce(state, { type: "run-test", test: t });
    assert.equal(state.tests.filter((t) => t.outcome === "FOUND").length, 1, id);
    assert.equal(state.tests.length, TEST_ORDER.length);
  }
});

test("a test cannot be run twice and fix is locked until evidence is found", () => {
  let state = started("loose-echo");
  state = reduce(state, { type: "run-test", test: "rail-load" });
  assert.equal(reduce(state, { type: "run-test", test: "rail-load" }), state);
  assert.equal(reduce(state, { type: "go-fix" }).stage, "investigate");
  state = reduce(state, { type: "run-test", test: "wiggle" });
  assert.equal(reduce(state, { type: "go-fix" }).stage, "fix");
});

test("the wiggle test changes the evidence for a loose connection", () => {
  const state = reduce(started("loose-echo"), { type: "run-test", test: "wiggle" });
  const [runResult] = state.tests;
  assert.equal(runResult.outcome, "FOUND");
  assert.ok(runResult.during.echo.dropouts_per_min > runResult.before.echo.dropouts_per_min);
});

test("a wrong fix stays weird; the supported fix verifies and unlocks the passport", () => {
  for (const id of FAULT_ORDER) {
    const right = FAULTS[id].fix;
    const wrong = FIX_ORDER.find((f) => f !== right);
    const finder = TEST_ORDER.find((t) => reduce(started(id), { type: "run-test", test: t }).tests[0].outcome === "FOUND");
    let state = reduce(reduce(started(id), { type: "run-test", test: finder }), { type: "go-fix" });
    state = reduce(state, { type: "apply-fix", fix: wrong });
    assert.equal(verifyHeadline(state), "STILL WEIRD.", id);
    assert.equal(reduce(state, { type: "passport", now: 2 }).stage, "verify", "passport stays locked");
    state = reduce(reduce(state, { type: "retry" }), { type: "apply-fix", fix: right });
    assert.equal(verifyHeadline(state), "NOT WEIRD ANYMORE.", id);
    assert.equal(verifies(current(state)), true);
    state = reduce(state, { type: "passport", now: 2 });
    assert.equal(state.stage, "passport");
    const record = incidentRecord(state);
    assert.equal(record.simulated, true);
    assert.equal(record.verification, "RESOLVED");
    assert.equal(record.fix_attempts.length, 2);
  }
});

test("mystery faults stay hidden until a test finds evidence", () => {
  let state = run({ type: "start", now: 1 }, { type: "make-weird", fault: "mystery", random: () => 0.99 });
  assert.equal(state.fault, "timing-drift");
  assert.equal(state.mystery, true);
  assert.equal(revealed(state), false);
  state = reduce(state, { type: "run-test", test: "wiggle" });
  assert.equal(revealed(state), false);
  state = reduce(state, { type: "run-test", test: "trigger-timing" });
  assert.equal(found(state), true);
  assert.equal(revealed(state), true);
  assert.equal(pickMystery(() => 0), "loose-echo");
});

test("actions out of order are ignored", () => {
  assert.equal(reduce(initialState, { type: "run-test", test: "wiggle" }), initialState);
  assert.equal(reduce(initialState, { type: "apply-fix", fix: "reseat-echo" }), initialState);
  assert.equal(reduce(initialState, { type: "make-weird", fault: "loose-echo" }), initialState);
});

test("hints point at the failing probe without naming the answer, and stop after evidence", () => {
  for (const id of FAULT_ORDER) {
    let state = started(id);
    const h1 = hint(state, 1), h2 = hint(state, 2);
    assert.ok(h1 && h2, id);
    for (const t of TEST_ORDER) assert.ok(!h1.includes(demo.TESTS[t].label), `${id} hint names a test`);
    state = reduce(reduce(reduce(state, { type: "hint" }), { type: "hint" }), { type: "hint" });
    assert.equal(state.hints, 2);
    const finder = TEST_ORDER.find((t) => reduce(started(id), { type: "run-test", test: t }).tests[0].outcome === "FOUND");
    state = reduce(state, { type: "run-test", test: finder });
    assert.equal(reduce(state, { type: "hint" }), state);
  }
});

test("the on-screen capture follows the latest test", () => {
  let state = started("loose-echo");
  assert.equal(latestCapture(state).snapshot, FAULTS["loose-echo"].symptom);
  state = reduce(state, { type: "run-test", test: "wiggle" });
  assert.equal(latestCapture(state).snapshot.echo.dropouts_per_min, 27);
  assert.match(latestCapture(state).label, /wiggle/);
});

test("back walks one step at a time and never skips past a verified fix", () => {
  let state = started("loose-echo");
  state = reduce(state, { type: "run-test", test: "wiggle" });
  state = reduce(state, { type: "go-fix" });
  assert.equal(reduce(state, { type: "back" }).stage, "investigate");
  assert.equal(reduce(state, { type: "back" }).tests.length, 1, "keeps the tests already run");
  const wrong = reduce(state, { type: "apply-fix", fix: "stiffen-supply" });
  assert.equal(canGoBack(wrong), true);
  assert.equal(reduce(wrong, { type: "back" }).stage, "fix");
  const right = reduce(state, { type: "apply-fix", fix: "reseat-echo" });
  assert.equal(canGoBack(right), false);
  assert.equal(reduce(right, { type: "back" }), right);
  const investigating = started("missing-echo");
  const chose = reduce(investigating, { type: "back" });
  assert.equal(chose.stage, "choose");
  assert.equal(chose.fault, null);
  assert.equal(reduce(chose, { type: "back" }).stage, "intro");
  assert.equal(canGoBack(initialState), false);
});
