import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./judge-demo.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const demo = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);
const { reduce, initialState, checks, verifies, current, revealed, found, hint, rank, waveform, incidentRecord, pickMystery, canGoBack, BASELINE, FAULTS, FAULT_ORDER, TEST_ORDER, FIX_ORDER, SCORE, TESTS } = demo;

const run = (...actions) => actions.reduce(reduce, initialState);
const started = (fault) => run({ type: "start", now: 1 }, { type: "make-weird", fault });
const finderFor = (id) => TEST_ORDER.find((t) => reduce(started(id), { type: "run-test", test: t }).tests[0].outcome === "FOUND");

test("the healthy baseline passes every check and verifies", () => {
  assert.deepEqual(checks(BASELINE).map((c) => c.status), ["pass", "pass", "pass", "pass"]);
  assert.equal(verifies(BASELINE), true);
});

test("every fault fails at least one check and does not verify", () => {
  for (const id of FAULT_ORDER) {
    assert.ok(checks(FAULTS[id].symptom).some((c) => c.status === "fail"), id);
    assert.equal(verifies(FAULTS[id].symptom), false, id);
  }
});

test("exactly one test finds each fault", () => {
  for (const id of FAULT_ORDER) {
    let state = started(id);
    for (const t of TEST_ORDER) state = reduce(state, { type: "run-test", test: t });
    assert.equal(state.tests.filter((t) => t.outcome === "FOUND").length, 1, id);
  }
});

test("inspecting probes is free; tests and hints cost points once", () => {
  let state = started("loose-echo");
  state = reduce(reduce(state, { type: "inspect", probe: "P1" }), { type: "inspect", probe: "P1" });
  assert.deepEqual(state.inspected, ["P1"]);
  assert.equal(state.score, SCORE.start);
  state = reduce(state, { type: "run-test", test: "rail-load" });
  assert.equal(state.score, SCORE.start - SCORE.test["rail-load"]);
  assert.equal(reduce(state, { type: "run-test", test: "rail-load" }), state, "no double charge");
  state = reduce(reduce(reduce(state, { type: "hint" }), { type: "hint" }), { type: "hint" });
  assert.equal(state.hints, 2);
  assert.equal(state.score, SCORE.start - SCORE.test["rail-load"] - 2 * SCORE.hint);
});

test("a found test does not reveal a mystery; only a correct call does", () => {
  let state = run({ type: "start", now: 1 }, { type: "make-weird", fault: "mystery", random: () => 0.99 });
  assert.equal(state.fault, "timing-drift");
  state = reduce(state, { type: "run-test", test: "trigger-timing" });
  assert.equal(found(state), true);
  assert.equal(revealed(state), false);
  state = reduce(state, { type: "go-call" });
  state = reduce(state, { type: "call", fault: "loose-echo" });
  assert.equal(state.stage, "call");
  assert.equal(revealed(state), false);
  assert.equal(state.score, SCORE.start - SCORE.test["trigger-timing"] - SCORE.wrongCall);
  assert.equal(reduce(state, { type: "call", fault: "loose-echo" }), state, "same wrong call is not charged twice");
  state = reduce(state, { type: "call", fault: "timing-drift" });
  assert.equal(state.stage, "fix");
  assert.equal(revealed(state), true);
});

test("a wrong fix costs points and stays weird; the right fix verifies and ends the case", () => {
  for (const id of FAULT_ORDER) {
    const right = FAULTS[id].fix;
    const wrong = FIX_ORDER.find((f) => f !== right);
    let state = reduce(reduce(reduce(started(id), { type: "run-test", test: finderFor(id) }), { type: "go-call" }), { type: "call", fault: id });
    const before = state.score;
    state = reduce(state, { type: "apply-fix", fix: wrong, now: 5 });
    assert.equal(state.stage, "fix", id);
    assert.equal(state.score, before - SCORE.wrongFix);
    assert.equal(verifies(current(state)), false);
    state = reduce(state, { type: "apply-fix", fix: right, now: 9 });
    assert.equal(state.stage, "result", id);
    assert.equal(verifies(current(state)), true);
    const record = incidentRecord(state);
    assert.equal(record.simulated, true);
    assert.equal(record.verification, "RESOLVED");
    assert.equal(record.fix_attempts.length, 2);
    assert.equal(record.score, state.score);
  }
});

test("a sharp player who reads the scope and calls it outright scores 100", () => {
  let state = started("missing-echo");
  for (const p of ["P1", "P2", "P3"]) state = reduce(state, { type: "inspect", probe: p });
  state = reduce(reduce(state, { type: "go-call" }), { type: "call", fault: "missing-echo" });
  state = reduce(state, { type: "apply-fix", fix: "reconnect-echo", now: 3 });
  assert.equal(state.score, 100);
  assert.equal(rank(state.score).stars, 3);
});

test("score never goes below zero and ranks map to stars", () => {
  let state = reduce(started("loose-echo"), { type: "go-call" });
  for (const f of ["unstable-power", "missing-echo", "timing-drift"]) state = reduce(state, { type: "call", fault: f });
  assert.equal(state.score, 25);
  assert.equal(rank(90).stars, 3);
  assert.equal(rank(70).stars, 2);
  assert.equal(rank(10).stars, 1);
});

test("each fault leaves a visible fingerprint on the scope", () => {
  const flat = (xs) => new Set(xs).size === 1;
  assert.ok(flat(waveform(FAULTS["missing-echo"].symptom, "P3", 200)), "missing echo is a flat line");
  assert.ok(!flat(waveform(BASELINE, "P3", 200)), "healthy echo pulses");
  const width = (s) => waveform(s, "P2", 320).filter((v) => v > 0.5).length;
  assert.ok(width(FAULTS["timing-drift"].symptom) < width(BASELINE) / 3, "short TRIG pulses look thin");
  const lowest = (s) => Math.min(...waveform(s, "P1", 320));
  assert.ok(lowest(FAULTS["unstable-power"].symptom) < lowest(BASELINE) - 0.2, "rail visibly sags");
  const high = (s) => waveform(s, "P3", 320).filter((v) => v > 0.5).length;
  assert.ok(high(FAULTS["loose-echo"].symptom) < high(BASELINE), "loose echo has gaps");
});

test("mystery avoids already-solved faults when it can", () => {
  assert.equal(pickMystery(() => 0, ["loose-echo"]), "unstable-power");
  assert.equal(pickMystery(() => 0, FAULT_ORDER), "loose-echo");
});

test("hints never name a test", () => {
  for (const id of FAULT_ORDER) {
    const state = started(id);
    for (const level of [1, 2]) for (const t of TEST_ORDER) assert.ok(!hint(state, level).includes(TESTS[t].label), id);
  }
});

test("back and out-of-order actions are safe", () => {
  assert.equal(reduce(initialState, { type: "run-test", test: "wiggle" }), initialState);
  assert.equal(reduce(initialState, { type: "call", fault: "loose-echo" }), initialState);
  let state = reduce(started("loose-echo"), { type: "go-call" });
  assert.equal(canGoBack(state), true);
  assert.equal(reduce(state, { type: "back" }).stage, "investigate");
  state = reduce(state, { type: "call", fault: "loose-echo" });
  assert.equal(canGoBack(state), false);
  assert.equal(reduce(started("loose-echo"), { type: "back" }).stage, "choose");
});
