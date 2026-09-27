import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";
import ts from "typescript";

// Exercise the real modal's event handlers with isolated hook state and API
// spies. HTTP integration tests separately cover persistence and validation.
const require = createRequire(import.meta.url);
const source = readFileSync(new URL("../app/project-workflow.tsx", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 } }).outputText;
function harness() {
  const states = []; let cursor = 0;
  const calls = []; const completed = [];
  const result = { project: { id: "physical-project" }, analysis: {}, profile: { id: "physical-project" } };
  const api = new Proxy({}, { get: (_, method) => async (...args) => {
    calls.push([method, ...args]);
    return method === "createProject" ? result.project : result;
  } });
  const hooks = {
    useState(initial) { const i = cursor++; if (!(i in states)) states[i] = typeof initial === "function" ? initial() : initial; return [states[i], (v) => { states[i] = typeof v === "function" ? v(states[i]) : v; }]; },
    useRef: () => ({ current: null }), useEffect() {}, useMemo: (fn) => fn(),
  };
  const exports = {};
  const context = { exports, TextEncoder, require(id) {
    if (id === "react") return hooks;
    if (id === "react/jsx-runtime") return require(id);
    if (id === "@/lib/api") return { projectApi: api, githubApi: {}, ApiError: class extends Error {} };
    if (id === "./github-connection") return { useGitHubStatus: () => ({ status: { configured: true, connected: true }, refresh() {} }) };
    return new Proxy({}, { get: (_, key) => String(key) });
  } };
  vm.runInNewContext(compiled, context);
  let tree;
  function render() { cursor = 0; tree = exports.NewProjectModal({ onClose() {}, onComplete: (r) => completed.push(r), onLoadDemo() {}, onOpenProject() {} }); }
  function nodes(node) { if (!node || typeof node !== "object") return []; if (Array.isArray(node)) return node.flatMap((n) => nodes(n)); return [node, ...nodes(node.props?.children)]; }
  function find(predicate) { const node = nodes(tree).find(predicate); assert.ok(node, "UI control exists"); return node; }
  function button(text) { return find((n) => n.type === "button" && n.props.children === text); }
  function local() { button("Upload local project").props.onClick(); render(); }
  async function submit() { await find((n) => n.type === "form").props.onSubmit({ preventDefault() {} }); render(); }
  render();
  return { render, find, button, local, submit, calls, completed, states, result };
}

test("local photo and source upload converge to common profile without GitHub metadata", async () => {
  const h = harness(); h.local();
  const photo = { name: "circuit.jpg", size: 40 }; const code = { name: "target.ino", size: 80 };
  h.find((n) => n.type === "input" && n.props.accept?.includes("image/png")).props.onChange({ target: { files: [photo] } });
  h.find((n) => n.type === "input" && n.props.accept?.includes(".ino")).props.onChange({ target: { files: [code] } }); h.render();
  await h.submit();
  assert.deepEqual(h.calls.map((c) => c[0]), ["createProject", "uploadProjectImage", "uploadProjectCode", "analyzeProject"]);
  assert.equal(h.calls[0][1].repository, undefined);
  assert.ok(h.calls.slice(1).every((c) => c[1] === "physical-project"));
  assert.equal(h.completed[0], h.result);
});

test("GitHub import ignores retained local files and preserves repository sync path", async () => {
  const h = harness(); h.local();
  h.find((n) => n.type === "input" && n.props.accept?.includes(".ino")).props.onChange({ target: { files: [{ size: 20 }] } });
  h.button("Import from GitHub").props.onClick();
  // Repository state normally set by RepoPicker after its API effect.
  h.states[2] = { name: "rig", full_name: "team/rig", default_branch: "main" }; h.render();
  await h.submit();
  assert.deepEqual(h.calls.map((c) => c[0]), ["createProject", "syncRepository"]);
  assert.equal(h.calls[0][1].repository, "team/rig");
  assert.equal(h.completed[0].profile, h.result.profile);
});

test("local pasted code remains text and input method is locked after creation", async () => {
  const h = harness(); h.local();
  h.find((n) => n.type === "textarea" && n.props.rows === 6).props.onChange({ target: { value: "void setup() {}" } }); h.render();
  await h.submit();
  assert.deepEqual(h.calls.map((c) => c[0]), ["createProject", "submitPastedCode", "analyzeProject"]);
  assert.equal(h.calls[1][2], "void setup() {}");
  assert.equal(h.button("Import from GitHub").props.disabled, true);
});

test("empty and oversized local inputs never create a project", async () => {
  const h = harness(); h.local(); await h.submit(); assert.equal(h.calls.length, 0);
  h.find((n) => n.type === "input" && n.props.accept?.includes(".ino")).props.onChange({ target: { files: [{ size: 512 * 1024 + 1 }] } }); h.render();
  await h.submit(); assert.equal(h.calls.length, 0);
});
