import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";

const source = readFileSync(new URL("./calibration.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } }).outputText;
const { calibrationSteps, canSaveKnownGood, expectedText, observedText, knownGoodText, baselineStatusLabel, formatFrequency, formatDuration } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString("base64")}`);

const review = { status: "REVIEW_REQUIRED", can_save_known_good: true, provenance: "REAL_SERIAL", candidate_measurement_id: 42 };

test("Known Good can only be saved from a reviewable REAL SERIAL observation with explicit confirmation", () => {
  assert.equal(canSaveKnownGood(review, true), true);
  assert.equal(canSaveKnownGood(review, false), false, "explicit confirmation is required");
  assert.equal(canSaveKnownGood({ ...review, provenance: "SIMULATED" }, true), false, "simulated input never becomes physical Known Good");
  assert.equal(canSaveKnownGood({ ...review, status: "ATTENTION_REQUIRED", can_save_known_good: false }, true), false);
  assert.equal(canSaveKnownGood({ ...review, can_save_known_good: false }, true), false, "the API decides whether the observation is healthy enough");
  assert.equal(canSaveKnownGood(null, true), false);
});

test("calibration steps follow NOT CALIBRATED → observe → review → confirm → CALIBRATED", () => {
  assert.deepEqual(calibrationSteps("OBSERVING").map((step) => step.state), ["done", "current", "todo", "todo", "todo"]);
  assert.deepEqual(calibrationSteps("REVIEW_REQUIRED").map((step) => step.state), ["done", "done", "done", "current", "todo"]);
  assert.equal(calibrationSteps("ATTENTION_REQUIRED")[2].state, "blocked");
  assert.equal(calibrationSteps("SIMULATED_SOURCE")[0].state, "blocked");
  assert.ok(calibrationSteps("CALIBRATED").every((step) => step.state === "done"));
});

test("expected, observed and known good stay distinct", () => {
  const expected = { signal_type: "pulse", required: true, stable: false, min_frequency_hz: 3.4, max_frequency_hz: 4, min_pulse_width_us: 5, max_pulse_width_us: 20, max_dropouts: 0 };
  assert.equal(expectedText("TRIG", expected), "3.40 Hz–4.00 Hz · 5.0 µs–20.0 µs HIGH");
  assert.equal(expectedText("UNASSIGNED", expected), "Unassigned — no expectation");
  assert.equal(observedText({ windows: 10, stable_windows: 10, unreliable_windows: 0, max_dropouts: 0, active_windows: 10, min_frequency_hz: 3.77, max_frequency_hz: 3.77, min_pulse_width_us: 10, max_pulse_width_us: 10.1 }), "3.77 Hz · 10.0 µs–10.1 µs HIGH · 10/10 stable");
  assert.equal(observedText({ windows: 10, stable_windows: 0, unreliable_windows: 7, max_dropouts: 0, active_windows: 3 }), "Capture unreliable in 7/10 windows");
  assert.equal(knownGoodText(undefined), "Not calibrated");
  assert.equal(knownGoodText({ status: "USER_CONFIRMED_HEALTHY", min_voltage: 5.05, max_voltage: 5.08, window_count: 10 }), "5.05 V–5.08 V · 10 windows");
});

test("diagnosis shows KNOWN GOOD instead of a bare UNKNOWN", () => {
  assert.equal(baselineStatusLabel("UNKNOWN"), "Not calibrated yet");
  assert.equal(baselineStatusLabel("USER_CONFIRMED_HEALTHY", 10), "Known Good · 10 confirmed windows");
});

test("units scale without inventing precision", () => {
  assert.equal(formatFrequency(40000), "40.0 kHz");
  assert.equal(formatFrequency(50), "50.0 Hz");
  assert.equal(formatDuration(12270), "12.3 ms");
  assert.equal(formatDuration(486.5), "487 µs");
});
