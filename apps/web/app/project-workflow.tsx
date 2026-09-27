"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { motion } from "framer-motion";
import {
  AlertTriangle,
  Cable,
  Camera,
  Check,
  CheckCircle2,
  ExternalLink,
  FolderGit2,
  Github,
  Lock,
  RefreshCw,
  Search,
  ShieldCheck,
  WifiOff,
  X,
} from "lucide-react";
import type {
  AnalyzeProjectResponse,
  DemoSession,
  GitHubRepo,
  ProbePlan,
  Project,
  ProjectProfile,
} from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, githubApi, projectApi } from "@/lib/api";
import { CircuitMap } from "./circuit-map";
import { GitHubNotConfigured, startGitHubConnect, startGitHubInstall, useGitHubStatus } from "./github-connection";

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
  const [inputMethod, setInputMethod] = useState<"github" | "local">("github");
  const [image, setImage] = useState<File | null>(null);
  const [codeFile, setCodeFile] = useState<File | null>(null);
  const [pastedCode, setPastedCode] = useState("");
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
    if (inputMethod === "github" && !repo) { setError("Choose the GitHub repository that holds this project's firmware."); return; }
    setBusy(true);
    let projectID = createdID;
    try {
      if (inputMethod === "local") {
        if (!image && !codeFile && !pastedCode.trim()) throw new Error("Upload a photo or source code before analysis.");
        if (image && image.size > 5 * 1024 * 1024) throw new Error("The hardware photo exceeds the 5 MB limit.");
        if ((codeFile?.size ?? 0) > 512 * 1024 || new TextEncoder().encode(pastedCode).length > 512 * 1024) throw new Error("Source code exceeds the 512 KB limit.");
      }
      if (!projectID) {
        setStatus("Creating the project…");
        const project = await projectApi.createProject({ name: name.trim() || repo?.name || "My electronics project", description, controller, logic_voltage: logicVoltage, ...(inputMethod === "github" && repo ? { repository: repo.full_name } : {}) });
        projectID = project.id;
        setCreatedID(project.id);
      }
      if (inputMethod === "local") {
        setStatus("Uploading validated project data and analyzing evidence…");
        if (image) await projectApi.uploadProjectImage(projectID, image);
        if (codeFile) await projectApi.uploadProjectCode(projectID, codeFile);
        else if (pastedCode.trim()) await projectApi.submitPastedCode(projectID, pastedCode, controller.toLowerCase().includes("raspberry") ? "main.py" : "main.ino");
        onComplete(await projectApi.analyzeProject(projectID));
        return;
      }
      if (!repo) throw new Error("Choose a GitHub repository.");
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
          <div><span className="eyebrow">New project</span><h2 id="new-project-title">Create project</h2></div>
          <button type="button" className="icon-button" onClick={onClose} disabled={busy} aria-label="Close"><X size={18} /></button>
        </div>

        <div className="form-row" role="group" aria-label="Project input method">
          <button type="button" className={inputMethod === "github" ? "primary" : "secondary"} aria-pressed={inputMethod === "github"} disabled={busy || Boolean(createdID)} onClick={() => { setInputMethod("github"); setError(""); }}>Import from GitHub</button>
          <button type="button" className={inputMethod === "local" ? "primary" : "secondary"} aria-pressed={inputMethod === "local"} disabled={busy || Boolean(createdID)} onClick={() => { setInputMethod("local"); setError(""); }}>Upload local project</button>
        </div>
        {inputMethod === "github" && <div data-tw className="mb-4">
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
        </div>}

        {(inputMethod === "local" || connected) && <>
          <label>Project name<input required value={name} maxLength={120} disabled={busy} placeholder={repo?.name ?? "Pick a repository first"} onChange={(event) => { setName(event.target.value); setNameEdited(true); }} /></label>
          <label>Short description <small className="label-hint">optional, but helps define expected behavior</small><textarea rows={2} maxLength={2000} value={description} disabled={busy} onChange={(event) => setDescription(event.target.value)} placeholder="What should this project do?" /></label>
          <div className="form-row">
            <label>Controller<select value={controller} disabled={busy || Boolean(createdID)} onChange={(event) => setController(event.target.value)}><option>ESP32</option><option>Arduino Uno</option><option>Raspberry Pi Pico</option><option>Other controller</option></select></label>
            <label>Logic voltage<select value={logicVoltage} disabled={busy || Boolean(createdID)} onChange={(event) => setLogicVoltage(Number(event.target.value))}><option value={3.3}>3.3 V</option><option value={5}>5 V</option><option value={1.8}>1.8 V</option></select></label>
          </div>
          {inputMethod === "local" ? <>
            <label>Hardware photo · PNG/JPEG · max 5 MB<input type="file" accept="image/png,image/jpeg,.png,.jpg,.jpeg" disabled={busy} onChange={(event) => setImage(event.target.files?.[0] ?? null)} /></label>
            <label>Source code · max 512 KB<input type="file" accept=".ino,.cpp,.h,.hpp,.c,.py,.txt" disabled={busy || Boolean(pastedCode.trim())} onChange={(event) => setCodeFile(event.target.files?.[0] ?? null)} /></label>
            <label>Or paste source code<textarea rows={6} value={pastedCode} disabled={busy || Boolean(codeFile)} onChange={(event) => setPastedCode(event.target.value)} /></label>
            <p className="input-security">Uploads are validated and stored as data only. Uploaded code is never compiled or executed.</p>
          </> : <div className="input-security"><Camera size={15} /><span>Hardware photos come from the ReWeird camera on your bench as you build. Code is read from GitHub as text and never compiled or run.</span></div>}
        </>}

        {status && <div className="analysis-progress"><RefreshCw className="spin" size={15} /><span>{status}</span></div>}
        {error && <div className="form-error" role="alert"><AlertTriangle size={15} /><span>{error}{createdID && <> The project was created; <button type="button" className="underline" onClick={() => onOpenProject(createdID)}>open it</button> or try again.</>}</span></div>}
        <div className="modal-actions split-actions">
          <button type="button" className="text-button" onClick={onLoadDemo} disabled={busy}>Load Demo Project</button>
          <span />
          <button type="button" className="secondary" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="primary" type="submit" disabled={busy || (inputMethod === "github" && (!connected || !repo))}>{busy ? <RefreshCw className="spin" size={16} /> : <FolderGit2 size={16} />} {createdID ? "Try again" : "Create project"}</button>
        </div>
      </form>
    </div>
  );
}

export function ProbePlanView({ project, profile, plan, session, onConnected, simulated = false }: { simulated?: boolean; project: Project | null; profile: ProjectProfile | null; plan: ProbePlan | null; session: DemoSession | null; onConnected: () => Promise<void> }) {
  const [checked, setChecked] = useState(Boolean(plan?.connected));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => setChecked(Boolean(plan?.connected)), [plan]);
  if (!plan) return <section className="empty-state panel"><Cable size={32} /><h2>No probe plan yet</h2><p>Confirm a generated Project Profile first. ReWeird will then map GND and P1–P6 from that project—not from demo assumptions.</p></section>;
  async function proceed() { setBusy(true); setError(""); try { await onConnected(); } catch (caught) { setError(errorMessage(caught)); } finally { setBusy(false); } }
  return <>
    <section className="page-heading"><div><p className="kicker">Connect ReWeird</p><h1>Probe placement plan</h1><p>Generated from the confirmed profile for {project?.name ?? "this project"}. Verify voltage and polarity before touching the circuit.</p><p>Physical profile ID: <code>{plan.profile_id}</code>. Firmware and API must use this exact ID before live diagnostics.</p></div><span className="confirmed"><Check size={13} /> Profile confirmed</span></section>
    {error && <div className="form-error page-error"><AlertTriangle size={15} />{error}</div>}
    {profile && (
      <div className="mt-2 mb-6" data-tw>
        <CircuitMap profile={profile} plan={plan} session={session} demoMode={!project} onNavigate={() => {}} showActions={false} />
      </div>
    )}
    <motion.section className="probe-plan-grid" initial="hidden" animate="show" variants={{ hidden: {}, show: { transition: { staggerChildren: 0.06 } } }}>
      {plan.instructions.map((instruction) => <motion.article variants={{ hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: [0.16, 1, 0.3, 1] } } }} className={`probe-instruction ${instruction.probe === "GND" ? "ground" : ""}`} key={instruction.probe}><div className="probe-badge">{instruction.probe}</div><div><span className="eyebrow">{instruction.role}</span><h2>{instruction.target}</h2><p>{instruction.expected} · {instruction.signal_type}</p>{instruction.explanation && <small>{instruction.explanation}</small>}<div className="safety-warning"><ShieldCheck size={13} />{instruction.safe_warning}</div></div></motion.article>)}
      {["P1", "P2", "P3", "P4", "P5", "P6"].filter((probe) => !plan.instructions.some((step) => step.probe === probe)).map((probe) => <motion.article variants={{ hidden: { opacity: 0, y: 10 }, show: { opacity: 1, y: 0, transition: { duration: 0.3, ease: [0.16, 1, 0.3, 1] } } }} className="probe-instruction" key={probe}><div className="probe-badge">{probe}</div><div><span className="eyebrow">Unassigned</span><h2>Spare / disconnected</h2><p>No target node is assigned in this confirmed profile.</p><div className="safety-warning"><ShieldCheck size={13} />Leave this probe disconnected.</div></div></motion.article>)}
    </motion.section>
    <section className="connection-confirm"><label><input type="checkbox" checked={checked} onChange={(event) => setChecked(event.target.checked)} /><span><strong>{simulated ? "Use the simulated probe connections shown above" : "I connected the probes exactly as shown"}</strong><small>I verified circuit ground, voltage range, divider/level-shifter requirements, and that PATCH remains disconnected.</small></span></label><button className="primary" disabled={!checked || busy} onClick={proceed}>{busy ? <RefreshCw className="spin" size={16} /> : <CheckCircle2 size={16} />} {simulated ? "Open simulated diagnostics" : "Proceed to live diagnostics"}</button></section>
  </>;
}
