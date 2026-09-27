import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import ts from "typescript";
const code = fs.readFileSync(new URL("./patch-simulation.ts", import.meta.url), "utf8");
const output = ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
const exports = {}; new Function("exports", output)(exports);
test("PATCH practice cannot arm without approval and never escapes SIMULATED lifecycle", () => {
  let state = "PROPOSED";
  state = exports.patchPracticeNext(state, false); assert.equal(state, "VALIDATED");
  state = exports.patchPracticeNext(state, false); assert.equal(state, "AWAITING_APPROVAL");
  assert.equal(exports.patchPracticeNext(state, false), state);
  for (const expected of ["ARMED", "ACTIVE", "DISABLED", "RE_MEASURE", "VERIFY"]) { state = exports.patchPracticeNext(state, true); assert.equal(state, expected); }
  assert.equal(exports.patchPracticeNext(state, true), "VERIFY");
  assert.equal(exports.patchPracticeNext("CANCELLED", true), "CANCELLED");
  const component = fs.readFileSync(new URL("../app/simulated-patch.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(component, /patchApi|fetch\(|REAL_SERIAL|knownGood/);
  assert.match(component, /NO ELECTRICAL OUTPUT/);
});
test("physical approval UI binds digest and confirmation, never supplied execution claims", () => {
  const api = fs.readFileSync(new URL("./api.ts", import.meta.url), "utf8");
  assert.match(api, /digest: action.digest, confirm: true/);
  const component = fs.readFileSync(new URL("../app/patch-status.tsx", import.meta.url), "utf8");
  assert.match(component, /!confirm \|\| busy \|\| !qualified \|\| !status\?\.master_enabled/);
  for (const field of ["target_node", "patch_pin", "logic_level", "max_voltage", "duration_ms", "device_id", "profile_revision"]) assert.ok(component.includes(`pending.parameters.${field}`));
  assert.match(component, /Cancel \/ disable output/);
});
