"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  Cable,
  Check,
  CheckCircle2,
  ChevronRight,
  Code2,
  Cpu,
  Eye,
  FileCode2,
  Image as ImageIcon,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import type {
  AnalyzeProjectResponse,
  ProbePlan,
  ProfileComponent,
  ProfileConnection,
  ProfileConflict,
  Project,
  ProjectFactSource,
  ProjectProfile,
} from "@reweird/shared-types";
import { ApiError, projectApi } from "@/lib/api";

const sourceLabels: Record<ProjectFactSource, string> = {
  CODE_STATIC_ANALYSIS: "CODE",
  VISION_AI: "VISION",
  CATALOG: "CATALOG",
  USER: "USER",
  INFERRED: "INFERRED",
};

function SourceBadges({ sources = [] }: { sources?: ProjectFactSource[] }) {
  return <span className="source-badges">{sources.map((source) => <i key={source} className={`source-${source.toLowerCase()}`}>{sourceLabels[source]}</i>)}</span>;
}

function errorMessage(error: unknown): string {
  return error instanceof ApiError ? error.message : "The request failed. Check the backend and try again.";
}

export function NewProjectModal({
  onClose,
  onComplete,
  onLoadDemo,
}: {
  onClose: () => void;
  onComplete: (result: AnalyzeProjectResponse) => void;
  onLoadDemo: () => void;
}) {
  const [name, setName] = useState("My electronics project");
  const [description, setDescription] = useState("");
  const [controller, setController] = useState("ESP32");
  const [logicVoltage, setLogicVoltage] = useState(3.3);
  const [image, setImage] = useState<File | null>(null);
  const [codeFile, setCodeFile] = useState<File | null>(null);
  const [pastedCode, setPastedCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const previousFocus = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    formRef.current?.querySelector<HTMLInputElement>('input[required]')?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, []);

  function handleDialogKey(event: React.KeyboardEvent<HTMLFormElement>) {
    if (event.key === "Escape" && !busy) { event.preventDefault(); onClose(); }
    if (event.key !== "Tab") return;
    const controls = formRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled)');
    if (!controls?.length) return;
    const first = controls[0];
    const last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    if (!image && !codeFile && !pastedCode.trim()) {
      setError("Add a PNG/JPEG photo or upload/paste source code before analysis.");
      return;
    }
    if (image && image.size > 5 * 1024 * 1024) {
      setError("The hardware photo exceeds the 5 MB limit.");
      return;
    }
    if (codeFile && codeFile.size > 512 * 1024) {
      setError("The source file exceeds the 512 KB limit.");
      return;
    }
    setBusy(true);
    try {
      setStatus("Creating persisted project…");
      const project = await projectApi.createProject({ name, description, controller, logic_voltage: logicVoltage });
      if (image) {
        setStatus("Uploading and validating hardware photo…");
        await projectApi.uploadProjectImage(project.id, image);
      }
      if (pastedCode.trim()) {
        setStatus("Saving pasted code as text…");
        await projectApi.submitPastedCode(project.id, pastedCode, controller.toLowerCase().includes("raspberry") ? "main.py" : "main.ino");
      } else if (codeFile) {
        setStatus("Uploading and validating source code…");
        await projectApi.uploadProjectCode(project.id, codeFile);
      }
      setStatus("Parsing code, analyzing the image, and merging evidence…");
      onComplete(await projectApi.analyzeProject(project.id));
    } catch (caught) {
      setError(errorMessage(caught));
      setStatus("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={busy ? undefined : onClose}>
      <form ref={formRef} className="modal project-modal" role="dialog" aria-modal="true" aria-labelledby="upload-project-title" onKeyDown={handleDialogKey} onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <div className="modal-head">
          <div><span className="eyebrow">01 / Project input</span><h2 id="upload-project-title">Create and analyze a project</h2></div>
          <button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="Close"><X size={18} /></button>
        </div>
        <label>Project name<input required value={name} maxLength={120} onChange={(event) => setName(event.target.value)} /></label>
        <label>Short description <small className="label-hint">optional, but helps define expected behavior</small><textarea rows={2} maxLength={2000} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="What should this project do?" /></label>
        <div className="form-row">
          <label>Controller<select value={controller} onChange={(event) => setController(event.target.value)}><option>ESP32</option><option>Arduino Uno</option><option>Raspberry Pi Pico</option><option>Other controller</option></select></label>
          <label>Logic voltage<select value={logicVoltage} onChange={(event) => setLogicVoltage(Number(event.target.value))}><option value={3.3}>3.3 V</option><option value={5}>5 V</option><option value={1.8}>1.8 V</option></select></label>
        </div>
        <div className="upload-grid">
          <label className={image ? "has-file" : ""}><ImageIcon size={20} /><span>{image?.name ?? "Hardware photo"}</span><small>PNG/JPG · max 5 MB</small><input type="file" accept="image/png,image/jpeg,.png,.jpg,.jpeg" disabled={busy} onChange={(event) => setImage(event.target.files?.[0] ?? null)} /></label>
          <label className={codeFile ? "has-file" : ""}><FileCode2 size={20} /><span>{codeFile?.name ?? "Project code"}</span><small>.ino .cpp .h .hpp .c .py .txt</small><input type="file" accept=".ino,.cpp,.h,.hpp,.c,.py,.txt" disabled={busy || Boolean(pastedCode.trim())} onChange={(event) => setCodeFile(event.target.files?.[0] ?? null)} /></label>
        </div>
        <div className="or-divider"><span>or paste code</span></div>
        <label>Source code<textarea className="code-input" rows={6} value={pastedCode} disabled={busy || Boolean(codeFile)} onChange={(event) => setPastedCode(event.target.value)} placeholder="#define LED_PIN 2&#10;void setup() { pinMode(LED_PIN, OUTPUT); }" /></label>
        <div className="input-security"><ShieldCheck size={15} /><span>Uploads are validated and stored as data. ReWeird never compiles, executes, or evaluates uploaded code.</span></div>
        {status && <div className="analysis-progress"><RefreshCw className="spin" size={15} /><span>{status}</span></div>}
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} />{error}</div>}
        <div className="modal-actions split-actions">
          <button type="button" className="text-button" onClick={onLoadDemo} disabled={busy}>Load Demo Project</button>
          <span />
          <button type="button" className="secondary" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="primary" type="submit" disabled={busy}>{busy ? <RefreshCw className="spin" size={16} /> : <Upload size={16} />} Analyze project</button>
        </div>
      </form>
    </div>
  );
}

function cloneProfile(profile: ProjectProfile): ProjectProfile {
  return JSON.parse(JSON.stringify(profile)) as ProjectProfile;
}

function blankExpected(behavior = "digital"): ProfileConnection["expected"] {
  return { signal_type: behavior.replaceAll("_", " "), required: true, stable: false, max_dropouts: 0 };
}

export function ProjectProfileView({
  project,
  profile,
  onSave,
  onConfirm,
}: {
  project: Project | null;
  profile: ProjectProfile | null;
  onSave: (profile: ProjectProfile) => Promise<ProjectProfile>;
  onConfirm: (profile: ProjectProfile) => Promise<void>;
}) {
  const [draft, setDraft] = useState<ProjectProfile | null>(profile ? cloneProfile(profile) : null);
  const [editing, setEditing] = useState(Boolean(profile && !profile.confirmed));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    setDraft(profile ? cloneProfile(profile) : null);
    setEditing(Boolean(profile && !profile.confirmed));
  }, [profile]);

  const unresolved = useMemo(() => draft?.conflicts?.filter((conflict) => conflict.requires_confirmation && !conflict.resolved) ?? [], [draft]);

  if (!draft) {
    return <section className="empty-state panel"><Cpu size={32} /><h2>No Project Profile yet</h2><p>Create and analyze a project, or load the demo profile.</p></section>;
  }

  function updateComponent(index: number, patch: Partial<ProfileComponent>) {
    setDraft((current) => current ? { ...current, components: current.components.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item) } : current);
  }

  function updateConnection(index: number, patch: Partial<ProfileConnection>) {
    setDraft((current) => current ? { ...current, connections: (current.connections ?? []).map((item, itemIndex) => itemIndex === index ? { ...item, ...patch, sources: [...new Set([...(item.sources ?? []), "USER" as const])] } : item) } : current);
  }

  function resolveConflict(conflict: ProfileConflict, value: string) {
    setDraft((current) => {
      if (!current) return current;
      const match = value.toUpperCase().match(/^GPIO\s*(\d{1,2})$/);
      return {
        ...current,
        conflicts: (current.conflicts ?? []).map((item) => item.id === conflict.id ? { ...item, resolution: value.replace(/\s/g, ""), resolved: Boolean(match) } : item),
        connections: (current.connections ?? []).map((connection) => connection.id === conflict.connection_id && match ? { ...connection, gpio: Number(match[1]), target: `${current.controller} GPIO${match[1]} / ${connection.component_name} ${connection.role}`, sources: [...new Set([...connection.sources, "USER" as const])] } : connection),
      };
    });
  }

  async function save() {
    setBusy(true); setError("");
    try { if (draft) setDraft(cloneProfile(await onSave(draft))); }
    catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(false); }
  }

  async function confirm() {
    setBusy(true); setError("");
    try { if (draft) await onConfirm(draft); }
    catch (caught) { setError(errorMessage(caught)); }
    finally { setBusy(false); }
  }

  return (
    <>
      <section className="page-heading">
        <div><p className="kicker">{draft.confirmed ? "Authoritative project context" : "Layer 2 · Draft understanding"}</p><h1>Project Profile</h1><p>{draft.confirmed ? "This user-confirmed profile is the intended-behavior model for diagnostics." : "Review every suggestion. Code, vision, catalog, inference, and user corrections remain distinguishable."}</p></div>
        <div className="heading-actions">
          {!draft.confirmed && <button className="secondary" onClick={save} disabled={busy}><Save size={16} /> Save corrections</button>}
          {!draft.confirmed && <button className="primary" onClick={confirm} disabled={busy || unresolved.length > 0 || draft.components.length === 0 || (draft.connections?.length ?? 0) === 0}>{busy ? <RefreshCw className="spin" size={16} /> : <Check size={16} />} Confirm profile</button>}
          {draft.confirmed && <span className="confirmed"><Check size={13} /> Human confirmed</span>}
        </div>
      </section>

      {error && <div className="form-error page-error" role="alert"><AlertTriangle size={15} />{error}</div>}
      <section className="profile-grid">
        <div className="panel profile-summary">
          <div className="project-visual"><Cpu size={46} /><span>{draft.controller}</span></div>
          <div className="profile-main-fields"><span className="eyebrow">{project ? "Uploaded project" : "Built-in demo"}</span><h2>{draft.project_name}</h2>
            {editing ? <textarea value={draft.expected_behavior} rows={3} onChange={(event) => setDraft({ ...draft, expected_behavior: event.target.value })} /> : <p>{draft.expected_behavior}</p>}
            <div className="spec-chips"><span>{draft.logic_voltage} V logic</span><span>{draft.components.length} components</span><span>{draft.connections?.length ?? 0} connections</span></div>
          </div>
        </div>
        <div className="panel evidence-status">
          <div className="panel-heading"><div><span className="eyebrow">Analysis sources</span><h2>What ReWeird knows</h2></div></div>
          <div className="analysis-source-row"><Code2 size={17} /><div><strong>Static code analysis</strong><span>{project?.analysis?.code.status ?? (project ? "Not run" : "Demo seed")}</span></div><em>{project?.analysis?.code.pins.length ?? draft.connections?.length ?? 0} pins</em></div>
          <div className="analysis-source-row"><Eye size={17} /><div><strong>Gemini Vision</strong><span>{project?.analysis?.vision.status ?? (project ? "Not run" : "Demo seed")}</span></div><em>{project?.analysis?.vision.components.length ?? 0} suggestions</em></div>
          {editing && <label className="inline-field">Logic voltage<input type="number" min="0.1" max="5.5" step="0.1" value={draft.logic_voltage} onChange={(event) => setDraft({ ...draft, logic_voltage: Number(event.target.value) })} /></label>}
        </div>
      </section>

      {(draft.conflicts?.length ?? 0) > 0 && <section className="panel conflict-panel"><div className="panel-heading"><div><span className="eyebrow">Evidence disagreements</span><h2>{unresolved.length ? `${unresolved.length} conflict${unresolved.length === 1 ? "" : "s"} need confirmation` : "Conflicts resolved"}</h2></div></div>{draft.conflicts?.map((conflict) => <div className={`conflict-card ${conflict.resolved ? "resolved" : ""}`} key={conflict.id}><div><strong>{conflict.field}</strong><span>{conflict.resolved ? `Resolved as ${conflict.resolution}` : "Code and vision proposed different values."}</span></div><div className="conflict-options">{conflict.options.map((option) => <button type="button" className={conflict.resolution === option.value ? "selected" : ""} key={`${option.source}-${option.value}`} disabled={!editing} onClick={() => resolveConflict(conflict, option.value)}><b>{option.value}</b><small>{sourceLabels[option.source]} · {Math.round(option.confidence * 100)}%</small></button>)}{editing && <input aria-label={`Manual resolution for ${conflict.field}`} placeholder="GPIO number" onBlur={(event) => event.target.value && resolveConflict(conflict, `GPIO${event.target.value}`)} />}</div></div>)}</section>}

      <section className="panel component-editor">
        <div className="panel-heading"><div><span className="eyebrow">Component catalog + detections</span><h2>Components</h2></div>{editing && <button className="text-button" onClick={() => setDraft({ ...draft, components: [...draft.components, { id: `user-component-${draft.components.length + 1}`, name: "New component", sources: ["USER"], confidence: 1, confirmed: false }] })}><Plus size={14} /> Add component</button>}</div>
        {draft.components.length === 0 ? <div className="inline-empty">No component detected. Add one before confirmation.</div> : draft.components.map((component, index) => <div className="component-edit-row" key={`${component.id}-${index}`}><Cpu size={18} /><div>{editing ? <input value={component.name} onChange={(event) => updateComponent(index, { name: event.target.value })} /> : <strong>{component.name}</strong>}<span>{component.interface_type || "Interface needs confirmation"}</span></div><SourceBadges sources={component.sources} /><span className="confidence-text">{Math.round((component.confidence ?? 0) * 100)}%</span>{editing && <button className="icon-button danger-button" aria-label={`Remove ${component.name}`} onClick={() => setDraft({ ...draft, components: draft.components.filter((_, itemIndex) => itemIndex !== index) })}><Trash2 size={14} /></button>}</div>)}
      </section>

      <section className="panel connection-editor">
        <div className="panel-heading"><div><span className="eyebrow">GPIO and signal roles</span><h2>Proposed connections</h2></div>{editing && <button className="text-button" onClick={() => { const next = (draft.connections?.length ?? 0) + 1; setDraft({ ...draft, connections: [...(draft.connections ?? []), { id: `user-connection-${next}`, component_name: draft.components[0]?.name ?? "Component", role: "SIGNAL", target: "Enter target node", direction: "unknown", behavior: "digital", expected: blankExpected(), confidence: 1, sources: ["USER"], required: true, confirmed: false }] }); }}><Plus size={14} /> Add connection</button>}</div>
        {(draft.connections?.length ?? 0) === 0 ? <div className="inline-empty">No signal connection was found. Add one before confirmation.</div> : <div className="connection-list">{draft.connections?.map((connection, index) => <div className="connection-row" key={connection.id}><div className="connection-title"><span>{connection.role}</span><SourceBadges sources={connection.sources} /></div><div className="connection-fields"><label>Component<input disabled={!editing} value={connection.component_name} onChange={(event) => updateConnection(index, { component_name: event.target.value })} /></label><label>Role<input disabled={!editing} value={connection.role} onChange={(event) => updateConnection(index, { role: event.target.value })} /></label><label>GPIO<input disabled={!editing} type="number" min="0" max="99" value={connection.gpio ?? ""} onChange={(event) => updateConnection(index, { gpio: event.target.value === "" ? undefined : Number(event.target.value) })} /></label><label>Behavior<select disabled={!editing} value={connection.behavior} onChange={(event) => updateConnection(index, { behavior: event.target.value, expected: { ...connection.expected, signal_type: event.target.value.replaceAll("_", " ") } })}><option value="digital_input">Digital input</option><option value="digital_output">Digital output</option><option value="digital_pulse">Digital pulse</option><option value="pulse_input">Pulse input</option><option value="pwm_output">PWM output</option><option value="analog_input">Analog voltage</option><option value="i2c_sda">I2C SDA</option><option value="i2c_scl">I2C SCL</option><option value="uart">UART</option></select></label><label className="wide-field">Target node<input disabled={!editing} value={connection.target} onChange={(event) => updateConnection(index, { target: event.target.value })} /></label></div>{editing && <button className="icon-button danger-button" aria-label={`Remove ${connection.role}`} onClick={() => setDraft({ ...draft, connections: draft.connections?.filter((_, itemIndex) => itemIndex !== index) })}><Trash2 size={14} /></button>}</div>)}</div>}
      </section>

      {!draft.confirmed && <section className="confirmation-callout"><ShieldCheck size={22} /><div><strong>Only you can confirm this profile</strong><span>Gemini suggestions remain VISION_AI. Confirmation records the reviewed model as user-authoritative and generates probe placement from these connections.</span></div><button className="primary" onClick={confirm} disabled={busy || unresolved.length > 0 || draft.components.length === 0 || (draft.connections?.length ?? 0) === 0}>Confirm Project Profile <ChevronRight size={16} /></button></section>}
    </>
  );
}

export function ProbePlanView({ project, plan, onConnected }: { project: Project | null; plan: ProbePlan | null; onConnected: () => Promise<void> }) {
  const [checked, setChecked] = useState(Boolean(plan?.connected));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => setChecked(Boolean(plan?.connected)), [plan]);
  if (!plan) return <section className="empty-state panel"><Cable size={32} /><h2>No probe plan yet</h2><p>Confirm a generated Project Profile first. ReWeird will then map GND and P1–P6 from that project—not from demo assumptions.</p></section>;
  async function proceed() { setBusy(true); setError(""); try { await onConnected(); } catch (caught) { setError(errorMessage(caught)); } finally { setBusy(false); } }
  return <>
    <section className="page-heading"><div><p className="kicker">Connect ReWeird</p><h1>Probe placement plan</h1><p>Generated from the confirmed profile for {project?.name ?? "this project"}. Verify voltage and polarity before touching the circuit.</p></div><span className="confirmed"><Check size={13} /> Profile confirmed</span></section>
    {error && <div className="form-error page-error"><AlertTriangle size={15} />{error}</div>}
    <section className="probe-plan-grid">{plan.instructions.map((instruction) => <article className={`probe-instruction ${instruction.probe === "GND" ? "ground" : ""}`} key={instruction.probe}><div className="probe-badge">{instruction.probe}</div><div><span className="eyebrow">{instruction.role}</span><h2>{instruction.target}</h2><p>{instruction.expected} · {instruction.signal_type}</p>{instruction.explanation && <small>{instruction.explanation}</small>}<div className="safety-warning"><ShieldCheck size={13} />{instruction.safe_warning}</div></div></article>)}</section>
    <section className="connection-confirm"><label><input type="checkbox" checked={checked} onChange={(event) => setChecked(event.target.checked)} /><span><strong>I connected the probes exactly as shown</strong><small>I verified circuit ground, voltage range, divider/level-shifter requirements, and that PATCH remains disconnected.</small></span></label><button className="primary" disabled={!checked || busy} onClick={proceed}>{busy ? <RefreshCw className="spin" size={16} /> : <CheckCircle2 size={16} />} Proceed to live diagnostics</button></section>
  </>;
}
