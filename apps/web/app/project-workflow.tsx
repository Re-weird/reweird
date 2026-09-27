"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  Cable,
  Camera,
  Check,
  CheckCircle2,
  ChevronRight,
  Code2,
  Cpu,
  ExternalLink,
  Eye,
  FolderGit2,
  Github,
  Lock,
  Plus,
  RefreshCw,
  Save,
  Search,
  ShieldCheck,
  Trash2,
  WifiOff,
  X,
} from "lucide-react";
import type {
  AnalyzeProjectResponse,
  DemoSession,
  GitHubRepo,
  ProbePlan,
  ProfileComponent,
  ProfileConnection,
  ProfileConflict,
  Project,
  ProjectFactSource,
  ProjectProfile,
} from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, githubApi, projectApi } from "@/lib/api";
import { CircuitMap } from "./circuit-map";
import { GitHubNotConfigured, startGitHubConnect, startGitHubInstall, useGitHubStatus } from "./github-connection";

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

function ago(ms: number) {
  const days = Math.floor((Date.now() - ms) / 86_400_000);
  if (days < 1) return "today";
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  return months < 12 ? `${months}mo ago` : `${Math.floor(months / 12)}y ago`;
}

function RepoPicker({ repos, selected, onSelect, disabled }: { repos: GitHubRepo[]; selected: GitHubRepo | null; onSelect: (repo: GitHubRepo) => void; disabled: boolean }) {
  const [query, setQuery] = useState("");
  const visible = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return repos.filter((repo) => !needle || repo.full_name.toLowerCase().includes(needle) || repo.description?.toLowerCase().includes(needle));
  }, [repos, query]);
  return (
    <div data-tw className="flex flex-col gap-2">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" strokeWidth={1.5} />
        <input aria-label="Search repositories" className="!pl-9" placeholder={`Search ${repos.length} repositories`} value={query} disabled={disabled} onChange={(event) => setQuery(event.target.value)} />
      </div>
      <div role="listbox" aria-label="Repositories" className="max-h-60 overflow-y-auto rounded-lg ring-1 ring-border">
        {visible.length === 0 ? <p className="px-3 py-6 text-center text-sm text-muted-foreground">No repository matches “{query}”.</p> : visible.map((repo) => {
          const active = selected?.id === repo.id;
          return (
            <button type="button" role="option" aria-selected={active} key={repo.id} disabled={disabled} onClick={() => onSelect(repo)}
              className={`flex w-full items-center gap-3 border-b border-border px-3 py-2.5 text-left transition-colors last:border-b-0 hover:bg-surface-2 disabled:cursor-not-allowed ${active ? "bg-surface-2" : ""}`}>
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-2">
                  <span className="truncate text-sm font-semibold text-foreground">{repo.full_name}</span>
                  {repo.private && <Lock className="size-3 shrink-0 text-muted-foreground" strokeWidth={1.5} aria-label="Private" />}
                  {repo.archived && <span className="shrink-0 rounded-full px-1.5 text-[10px] text-muted-foreground ring-1 ring-border">Archived</span>}
                </span>
                <span className="block truncate text-xs text-muted-foreground">{[repo.language, repo.default_branch, repo.pushed_at_ms > 0 ? `pushed ${ago(repo.pushed_at_ms)}` : ""].filter(Boolean).join(" · ")}</span>
              </span>
              {active && <Check className="size-4 shrink-0 text-signal" strokeWidth={2} />}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function NewProjectModal({
  onClose,
  onComplete,
  onLoadDemo,
  onOpenProject,
}: {
  onClose: () => void;
  onComplete: (result: AnalyzeProjectResponse) => void;
  onLoadDemo: () => void;
  /** Opens a project that was created but whose first analysis didn't finish. */
  onOpenProject: (projectID: string) => void;
}) {
  const { status: github, error: githubError, refresh: refreshGitHub } = useGitHubStatus();
  const [repos, setRepos] = useState<GitHubRepo[] | null>(null);
  const [reposError, setReposError] = useState("");
  const [repo, setRepo] = useState<GitHubRepo | null>(null);
  const [name, setName] = useState("");
  const [nameEdited, setNameEdited] = useState(false);
  const [description, setDescription] = useState("");
  const [controller, setController] = useState("ESP32");
  const [logicVoltage, setLogicVoltage] = useState(3.3);
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");
  const [createdID, setCreatedID] = useState("");
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const previousFocus = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    formRef.current?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, []);

  useEffect(() => {
    if (!github?.connected) return;
    let live = true;
    setReposError("");
    githubApi.repos()
      .then((list) => { if (live) setRepos(list.items); })
      .catch((cause) => { if (live) setReposError(errorMessage(cause)); });
    return () => { live = false; };
  }, [github?.connected]);

  function choose(next: GitHubRepo) {
    setRepo(next);
    if (!nameEdited) setName(next.name.replace(/[-_]+/g, " "));
    if (!description) setDescription(next.description ?? "");
  }

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
    if (!repo) { setError("Choose the GitHub repository that holds this project's firmware."); return; }
    setBusy(true);
    let projectID = createdID;
    try {
      if (!projectID) {
        setStatus("Creating the project…");
        const project = await projectApi.createProject({ name: name.trim() || repo.name, description, controller, logic_voltage: logicVoltage, repository: repo.full_name });
        projectID = project.id;
        setCreatedID(project.id);
      }
      setStatus(`Reading ${repo.default_branch} from ${repo.full_name} and analyzing the code…`);
      const result = await projectApi.syncRepository(projectID);
      if (!result.profile || !result.analysis) throw new Error("The repository was read, but no analysis was produced.");
      onComplete({ project: result.project, analysis: result.analysis, profile: result.profile });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "The project couldn't be analyzed.");
      setStatus("");
    } finally {
      setBusy(false);
    }
  }

  const connected = github?.configured && github.connected;
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={busy ? undefined : onClose}>
      <form ref={formRef} tabIndex={-1} className="modal project-modal" role="dialog" aria-modal="true" aria-labelledby="new-project-title" onKeyDown={handleDialogKey} onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <div className="modal-head">
          <div><span className="eyebrow">New project</span><h2 id="new-project-title">Start from a GitHub repository</h2></div>
          <button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="Close"><X size={18} /></button>
        </div>

        <div data-tw className="mb-4">
          {githubError ? (
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground"><WifiOff className="size-4" strokeWidth={1.5} /> {githubError}<Button type="button" size="sm" variant="outline" onClick={() => void refreshGitHub()}><RefreshCw /> Try again</Button></div>
          ) : !github ? (
            <Skeleton className="h-24 w-full bg-surface-2" />
          ) : !github.configured ? (
            <GitHubNotConfigured />
          ) : !github.connected ? (
            <div className="flex flex-col items-start gap-3 rounded-lg bg-surface-2 px-4 py-4 ring-1 ring-border">
              <p className="text-sm leading-relaxed text-foreground">Connect GitHub to choose the repository this project&apos;s firmware lives in.</p>
              <p className="text-xs leading-relaxed text-muted-foreground">ReWeird reads the code whenever you push to the default branch. You pick which repositories it can see; access is read-only.</p>
              <Button type="button" onClick={() => startGitHubConnect(github, true)} disabled={!github.authorize_url && !github.install_url}><Github /> Connect GitHub</Button>
            </div>
          ) : reposError ? (
            <p role="alert" className="text-sm text-fail">{reposError}</p>
          ) : !repos ? (
            <Skeleton className="h-60 w-full bg-surface-2" />
          ) : repos.length === 0 ? (
            <div className="flex flex-col items-start gap-3 rounded-lg bg-surface-2 px-4 py-4 ring-1 ring-border">
              <p className="text-sm text-foreground">ReWeird can&apos;t see any repositories on <strong>{github.account_login}</strong> yet.</p>
              {github.install_url && <Button type="button" size="sm" variant="outline" onClick={() => startGitHubInstall(github.install_url!, true)}><ExternalLink /> Choose repositories on GitHub</Button>}
            </div>
          ) : (
            <>
              <RepoPicker repos={repos} selected={repo} onSelect={choose} disabled={busy || Boolean(createdID)} />
              {github.install_url && <button type="button" className="mt-2 text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline" onClick={() => startGitHubInstall(github.install_url!, true)}>Missing a repository? Change which ones ReWeird can see</button>}
            </>
          )}
        </div>

        {connected && <>
          <label>Project name<input required value={name} maxLength={120} disabled={busy} placeholder={repo?.name ?? "Pick a repository first"} onChange={(event) => { setName(event.target.value); setNameEdited(true); }} /></label>
          <label>Short description <small className="label-hint">optional, but helps define expected behavior</small><textarea rows={2} maxLength={2000} value={description} disabled={busy} onChange={(event) => setDescription(event.target.value)} placeholder="What should this project do?" /></label>
          <div className="form-row">
            <label>Controller<select value={controller} disabled={busy || Boolean(createdID)} onChange={(event) => setController(event.target.value)}><option>ESP32</option><option>Arduino Uno</option><option>Raspberry Pi Pico</option><option>Other controller</option></select></label>
            <label>Logic voltage<select value={logicVoltage} disabled={busy || Boolean(createdID)} onChange={(event) => setLogicVoltage(Number(event.target.value))}><option value={3.3}>3.3 V</option><option value={5}>5 V</option><option value={1.8}>1.8 V</option></select></label>
          </div>
          <div className="input-security"><Camera size={15} /><span>Hardware photos come from the ReWeird camera on your bench as you build; there&apos;s nothing to upload. Code is read from GitHub as text and never compiled or run.</span></div>
        </>}

        {status && <div className="analysis-progress"><RefreshCw className="spin" size={15} /><span>{status}</span></div>}
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} /><span>{error}{createdID && <> The project was created; <button type="button" className="underline" onClick={() => onOpenProject(createdID)}>open it</button> or try again.</>}</span></div>}
        <div className="modal-actions split-actions">
          <button type="button" className="text-button" onClick={onLoadDemo} disabled={busy}>Load Demo Project</button>
          <span />
          <button type="button" className="secondary" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="primary" type="submit" disabled={busy || !connected || !repo}>{busy ? <RefreshCw className="spin" size={16} /> : <FolderGit2 size={16} />} {createdID ? "Try again" : "Create project"}</button>
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
  plan,
  session,
  onSave,
  onConfirm,
  onNavigate,
}: {
  project: Project | null;
  profile: ProjectProfile | null;
  plan: ProbePlan | null;
  session: DemoSession | null;
  onSave: (profile: ProjectProfile) => Promise<ProjectProfile>;
  onConfirm: (profile: ProjectProfile) => Promise<void>;
  onNavigate: (destination: "connect" | "live" | "diagnosis" | "guided" | "history") => void;
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

      <CircuitMap profile={draft} plan={plan} session={session} demoMode={!project} onNavigate={onNavigate} />

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
