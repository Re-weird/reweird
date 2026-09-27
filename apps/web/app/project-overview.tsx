"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  AlertTriangle, ArrowRight, Camera, Check, Code2, Cpu, ExternalLink, GitBranch, GitCommitHorizontal, Layers3, Plus,
  RefreshCw, Save, ShieldCheck, Trash2,
} from "lucide-react";
import type { DemoSession, ProbePlan, ProfileComponent, ProfileConflict, ProfileConnection, Project, ProjectFactSource, ProjectProfile } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api";
import { projectPath } from "@/lib/project-routes";
import { cn } from "@/lib/utils";
import { CircuitMap } from "./circuit-map";
import { PartThumb, partKind } from "./part-art";

type Destination = "connect" | "live" | "diagnosis" | "guided" | "history";

const EASE_OUT = [0.23, 1, 0.32, 1] as const;
const reveal = { hidden: { opacity: 0, transform: "translateY(8px)" }, show: { opacity: 1, transform: "translateY(0px)", transition: { duration: 0.28, ease: EASE_OUT } } };

const sourceStyle: Record<ProjectFactSource, { label: string; className: string }> = {
  CODE_STATIC_ANALYSIS: { label: "Code", className: "text-signal ring-signal/40" },
  VISION_AI: { label: "Vision", className: "text-warn ring-warn/40" },
  CATALOG: { label: "Catalog", className: "text-muted-foreground ring-border" },
  USER: { label: "You", className: "text-pass ring-pass/40" },
  INFERRED: { label: "Inferred", className: "text-subtle ring-border" },
};

// Behaviors are stored as "digital pulse" or "digital_pulse" depending on
// where they came from; compare on one form so the select never shows a
// different value from the one saved.
const behaviorOptions: [string, string][] = [
  ["digital_input", "Digital input"], ["digital_output", "Digital output"], ["digital_pulse", "Digital pulse"],
  ["pulse_input", "Pulse input"], ["pwm_output", "PWM output"], ["analog_input", "Analog voltage"],
  ["voltage_rail", "Power rail"], ["ground", "Ground"], ["i2c_sda", "I2C SDA"], ["i2c_scl", "I2C SCL"], ["uart", "UART"],
];
const behaviorKey = (value: string | undefined) => (value ?? "").trim().toLowerCase().replace(/[\s-]+/g, "_");
/** Signal family for the dot color in the connections table. */
function behaviorFamily(value: string | undefined) {
  const key = behaviorKey(value);
  if (key === "voltage_rail" || key === "ground" || key.includes("power")) return { dot: "bg-warn" };
  if (key.includes("pulse") || key.includes("pwm")) return { dot: "bg-pass" };
  if (key.startsWith("i2c") || key === "uart") return { dot: "bg-muted-foreground" };
  return { dot: "bg-signal" };
}
const behaviorLabel = (value: string | undefined) => behaviorOptions.find(([key]) => key === behaviorKey(value))?.[1] ?? (value || "Not set");

function SourceBadges({ sources = [] }: { sources?: ProjectFactSource[] }) {
  return (
    <span className="flex flex-wrap gap-1">
      {sources.map((source) => (
        <span key={source} className={cn("rounded-full px-1.5 py-px font-mono text-[10px] ring-1", sourceStyle[source].className)}>{sourceStyle[source].label}</span>
      ))}
    </span>
  );
}

function errorMessage(error: unknown): string {
  return error instanceof ApiError ? error.message : error instanceof Error ? error.message : "The request failed. Check the backend and try again.";
}

function cloneProfile(profile: ProjectProfile): ProjectProfile {
  return JSON.parse(JSON.stringify(profile)) as ProjectProfile;
}

function blankExpected(behavior = "digital"): ProfileConnection["expected"] {
  return { signal_type: behavior.replaceAll("_", " "), required: true, stable: false, max_dropouts: 0 };
}

function ago(ms: number) {
  const minutes = Math.max(0, Math.floor((Date.now() - ms) / 60_000));
  if (minutes < 60) return minutes < 1 ? "just now" : `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return hours < 24 ? `${hours}h ago` : `${Math.floor(hours / 24)}d ago`;
}

function SectionTitle({ eyebrow, title, action }: { eyebrow: string; title: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">{eyebrow}</p>
        <h2 className="mt-1 text-lg font-semibold tracking-tight text-foreground">{title}</h2>
      </div>
      {action}
    </div>
  );
}

/** Where the profile's facts came from: the repo commit, photo analysis, and the catalog. */
function Sources({ project, profile, onSync }: { project: Project | null; profile: ProjectProfile | null; onSync: () => Promise<void> }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const repo = project?.repository;
  const commit = repo?.last_commit;
  const pins = project?.analysis?.code.pins.length ?? (project ? 0 : profile?.connections?.length ?? 0);
  const visionCount = project?.analysis?.vision.components.length ?? 0;
  const catalogMatches = profile?.components.filter((component) => component.sources?.includes("CATALOG")).length ?? 0;

  async function sync() {
    setBusy(true); setError("");
    try { await onSync(); } catch (cause) { setError(errorMessage(cause)); } finally { setBusy(false); }
  }

  return (
    <motion.section variants={reveal} aria-label="Sources">
      <div className="grid grid-cols-1 divide-y divide-line-soft rounded-xl bg-surface ring-1 ring-border md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_minmax(0,1fr)] md:divide-x md:divide-y-0">
        <div className="min-w-0 p-4">
          <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground"><Code2 className="size-3.5" strokeWidth={1.5} /> Code</p>
          {repo ? (
            <>
              <a href={repo.html_url} target="_blank" rel="noreferrer" className="mt-2 inline-flex max-w-full items-center gap-1.5 truncate text-sm font-semibold text-foreground hover:text-signal">
                {repo.full_name} <ExternalLink className="size-3 shrink-0 text-subtle" strokeWidth={1.5} />
              </a>
              {commit ? (
                <a href={commit.html_url} target="_blank" rel="noreferrer" className="mt-1 flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground">
                  <GitCommitHorizontal className="size-3.5 shrink-0" strokeWidth={1.5} />
                  <span className="font-mono text-subtle">{commit.sha.slice(0, 7)}</span>
                  <span className="truncate">{commit.message}</span>
                  <span className="shrink-0 text-subtle">· {ago(commit.committed_at_ms)}</span>
                </a>
              ) : <p className="mt-1 text-xs text-muted-foreground">No commit analyzed yet.</p>}
              <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
                <span className="inline-flex items-center gap-1 font-mono text-[11px] text-subtle"><GitBranch className="size-3" strokeWidth={1.5} />{repo.default_branch}</span>
                {commit && <span className="font-mono text-[11px] text-subtle">{repo.analyzed_files?.length ?? 0} files · {pins} pins</span>}
                <Button size="sm" variant="outline" className="ml-auto h-7 active:scale-[0.97]" disabled={busy || repo.sync_status === "SYNCING"} onClick={() => void sync()}>
                  <RefreshCw className={cn(busy && "animate-spin")} /> {busy ? "Checking…" : "Check for new commits"}
                </Button>
              </div>
              {(error || (repo.sync_status !== "SYNCED" && repo.sync_error)) && <p role="alert" className="mt-2 text-xs text-fail">{error || repo.sync_error}</p>}
            </>
          ) : (
            <p className="mt-2 text-sm text-muted-foreground">{project ? project.code ? `Uploaded ${project.code.filename} · ${pins} pins` : "No repository linked." : `Built-in demo code · ${pins} pins`}</p>
          )}
        </div>
        <div className="p-4">
          <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground"><Camera className="size-3.5" strokeWidth={1.5} /> Photos</p>
          <p className="mt-2 text-sm text-foreground">{project?.image ? `${visionCount} part${visionCount === 1 ? "" : "s"} seen` : "Bench camera"}</p>
          <p className="mt-1 text-xs text-muted-foreground">{project?.image ? project.analysis?.vision.status ?? "Not analyzed" : "Photos arrive from the camera as you build."}</p>
        </div>
        <div className="p-4">
          <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground"><Layers3 className="size-3.5" strokeWidth={1.5} /> Catalog</p>
          <p className="mt-2 text-sm text-foreground">{catalogMatches} of {profile?.components.length ?? 0} parts matched</p>
          <p className="mt-1 text-xs text-muted-foreground">Pinouts and signal rules from known parts.</p>
        </div>
      </div>
    </motion.section>
  );
}

function ConflictList({ conflicts, editing, onResolve }: { conflicts: ProfileConflict[]; editing: boolean; onResolve: (conflict: ProfileConflict, value: string) => void }) {
  const open = conflicts.filter((conflict) => conflict.requires_confirmation && !conflict.resolved).length;
  return (
    <motion.section variants={reveal} aria-labelledby="conflicts-title" className="rounded-xl p-4 ring-1 ring-warn/40">
      <p className="flex items-center gap-2 text-sm font-semibold text-foreground" id="conflicts-title">
        <AlertTriangle className={cn("size-4", open ? "text-warn" : "text-pass")} strokeWidth={1.5} />
        {open ? `${open} disagreement${open === 1 ? "" : "s"} to settle before confirming` : "All disagreements settled"}
      </p>
      <div className="mt-3 space-y-3">
        {conflicts.map((conflict) => (
          <div key={conflict.id} className="flex flex-wrap items-center justify-between gap-3 border-t border-line-soft pt-3">
            <div>
              <p className="text-sm font-medium text-foreground">{conflict.field}</p>
              <p className="text-xs text-muted-foreground">{conflict.resolved ? `Set to ${conflict.resolution}` : "Code and photo suggest different values."}</p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {conflict.options.map((option) => {
                const chosen = conflict.resolution === option.value;
                return (
                  <button type="button" key={`${option.source}-${option.value}`} disabled={!editing} onClick={() => onResolve(conflict, option.value)}
                    className={cn("rounded-md px-3 py-1.5 text-left ring-1 transition-[background-color,box-shadow,transform] duration-150 ease-out active:scale-[0.97] disabled:opacity-60", chosen ? "bg-surface-2 ring-signal" : "ring-border hover:bg-surface-2")}>
                    <span className="block font-mono text-sm text-foreground">{option.value}</span>
                    <span className="block text-[10px] text-subtle">{sourceStyle[option.source].label} · {Math.round(option.confidence * 100)}%</span>
                  </button>
                );
              })}
              {editing && <input aria-label={`Other GPIO for ${conflict.field}`} placeholder="GPIO #" inputMode="numeric" className="h-9 w-20 rounded-md bg-background px-2 font-mono text-sm text-foreground ring-1 ring-border outline-none focus:ring-signal" onBlur={(event) => event.target.value && onResolve(conflict, `GPIO${event.target.value}`)} />}
            </div>
          </div>
        ))}
      </div>
    </motion.section>
  );
}

const fieldClass = "h-8 w-full rounded-md bg-background px-2 text-sm text-foreground ring-1 ring-border outline-none transition-shadow duration-150 focus:ring-signal disabled:opacity-60";

export function ProjectOverviewView({
  project, profile, plan, session, onSave, onConfirm, onSync, onNavigate,
}: {
  project: Project | null;
  profile: ProjectProfile | null;
  plan: ProbePlan | null;
  session: DemoSession | null;
  onSave: (profile: ProjectProfile) => Promise<ProjectProfile>;
  onConfirm: (profile: ProjectProfile) => Promise<void>;
  onSync: () => Promise<void>;
  onNavigate: (destination: Destination) => void;
}) {
  const reduce = useReducedMotion();
  const [draft, setDraft] = useState<ProjectProfile | null>(profile ? cloneProfile(profile) : null);
  const [busy, setBusy] = useState<"save" | "confirm" | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => { setDraft(profile ? cloneProfile(profile) : null); }, [profile]);

  const editing = Boolean(draft && !draft.confirmed);
  const unresolved = useMemo(() => draft?.conflicts?.filter((conflict) => conflict.requires_confirmation && !conflict.resolved) ?? [], [draft]);
  const connections = draft?.connections ?? [];
  const blockers = draft ? [
    unresolved.length ? `${unresolved.length} disagreement${unresolved.length === 1 ? "" : "s"} to settle` : "",
    draft.components.length === 0 ? "add at least one component" : "",
    connections.length === 0 ? "add at least one connection" : "",
  ].filter(Boolean) : [];

  if (!draft) {
    const repo = project?.repository;
    return (
      <div data-tw className="space-y-8">
        <SectionTitle eyebrow="Overview" title="Project profile" />
        <Sources project={project} profile={null} onSync={onSync} />
        <div className="flex max-w-lg flex-col items-start gap-3 py-6">
          <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border"><Cpu className="size-5" strokeWidth={1.5} /></span>
          <h3 className="text-base font-semibold text-foreground">No profile yet</h3>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {repo ? `ReWeird builds the profile from ${repo.full_name}. Use Check for new commits above to read ${repo.default_branch} now.` : "This project has no linked repository, so there's no code to analyze. Create a new project from a GitHub repository to get a profile."}
          </p>
        </div>
      </div>
    );
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
    setBusy("save"); setError(""); setSaved(false);
    try { if (draft) { setDraft(cloneProfile(await onSave(draft))); setSaved(true); } }
    catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(null); }
  }
  async function confirm() {
    setBusy("confirm"); setError("");
    try { if (draft) await onConfirm(draft); }
    catch (cause) { setError(errorMessage(cause)); }
    finally { setBusy(null); }
  }

  return (
    <motion.div data-tw initial={reduce ? false : "hidden"} animate="show" variants={{ show: { transition: { staggerChildren: 0.05 } } }} className="space-y-10">
      {/* Status + expected behavior */}
      <motion.header variants={reveal} className="grid grid-cols-1 gap-6 border-b border-border pb-8 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">{project ? "Overview" : "Built-in demo · overview"}</p>
            <span className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-mono text-[10px] uppercase ring-1", draft.confirmed ? "text-pass ring-pass/40" : "text-warn ring-warn/40")}>
              {draft.confirmed ? <ShieldCheck className="size-3" strokeWidth={1.5} /> : <AlertTriangle className="size-3" strokeWidth={1.5} />}
              {draft.confirmed ? `Confirmed · v${draft.version}` : "Draft · needs your review"}
            </span>
          </div>
          <h1 className="mt-3 text-sm font-medium text-muted-foreground">Expected behavior</h1>
          {editing ? (
            <textarea value={draft.expected_behavior} rows={2} maxLength={2000} onChange={(event) => setDraft({ ...draft, expected_behavior: event.target.value })}
              className="mt-1.5 w-full max-w-[70ch] resize-y rounded-md bg-background px-3 py-2 text-lg leading-snug text-foreground ring-1 ring-border outline-none transition-shadow duration-150 focus:ring-signal" />
          ) : (
            <p className="mt-1.5 max-w-[70ch] text-xl leading-snug font-medium tracking-tight text-foreground">{draft.expected_behavior || "Not described."}</p>
          )}
          <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[11px] text-subtle">
            <span>{draft.controller}</span>
            {editing ? (
              <label className="inline-flex items-center gap-1.5">Logic
                <input type="number" min="0.1" max="5.5" step="0.1" value={draft.logic_voltage} onChange={(event) => setDraft({ ...draft, logic_voltage: Number(event.target.value) })} className="h-6 w-14 rounded bg-background px-1.5 text-foreground ring-1 ring-border outline-none focus:ring-signal" /> V
              </label>
            ) : <span>{draft.logic_voltage} V logic</span>}
            <span>{draft.components.length} component{draft.components.length === 1 ? "" : "s"}</span>
            <span>{connections.length} connection{connections.length === 1 ? "" : "s"}</span>
          </p>
        </div>
        <div className="flex flex-col items-start gap-2 lg:items-end">
          {editing ? (
            <>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" className="active:scale-[0.97]" onClick={() => void save()} disabled={busy !== null}><Save /> {busy === "save" ? "Saving…" : "Save corrections"}</Button>
                <Button size="sm" className="active:scale-[0.97]" onClick={() => void confirm()} disabled={busy !== null || blockers.length > 0}>
                  {busy === "confirm" ? <RefreshCw className="animate-spin" /> : <Check />} Confirm profile
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">{blockers.length ? `To confirm: ${blockers.join(", ")}.` : "Only you can confirm. It locks the profile and generates the probe plan."}</p>
            </>
          ) : project ? (
            <Button asChild size="sm" variant="outline" className="active:scale-[0.97]"><Link href={projectPath(project.id, "probe-setup")}>Open probe setup <ArrowRight /></Link></Button>
          ) : null}
          <AnimatePresence>
            {saved && !error && (
              <motion.p initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.15 }} className="text-xs text-pass">Corrections saved.</motion.p>
            )}
          </AnimatePresence>
          {error && <p role="alert" className="max-w-sm text-xs text-fail">{error}</p>}
        </div>
      </motion.header>

      <Sources project={project} profile={draft} onSync={onSync} />

      {(draft.conflicts?.length ?? 0) > 0 && <ConflictList conflicts={draft.conflicts ?? []} editing={editing} onResolve={resolveConflict} />}

      <motion.div variants={reveal}>
        <CircuitMap profile={draft} plan={plan} session={session} demoMode={!project} onNavigate={onNavigate} showActions={false} />
      </motion.div>

      <div className="grid grid-cols-1 gap-10 border-t border-border pt-10 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.35fr)] xl:gap-0">
        {/* Components */}
        <motion.section variants={reveal} aria-labelledby="components-title" className="min-w-0 xl:pr-10">
          <SectionTitle eyebrow={`Parts · ${draft.components.length}`} title="Components" action={editing && (
            <Button variant="ghost" size="sm" onClick={() => setDraft({ ...draft, components: [...draft.components, { id: `user-component-${draft.components.length + 1}`, name: "New component", sources: ["USER"], confidence: 1, confirmed: false }] })}><Plus /> Add</Button>
          )} />
          {draft.components.length === 0 ? (
            <p className="mt-4 rounded-lg border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">No component detected. Add one before confirming.</p>
          ) : (
            <ul className="mt-5 space-y-3">
              {draft.components.map((component, index) => {
                const confidence = Math.round((component.confidence ?? 0) * 100);
                return (
                  <li key={`${component.id}-${index}`} className="flex items-center gap-4 rounded-xl bg-surface p-3 ring-1 ring-border">
                    <span className="grid h-16 w-24 shrink-0 place-items-center overflow-hidden rounded-lg bg-[var(--scope-bg)] ring-1 ring-line-soft">
                      <PartThumb kind={partKind(component.id, component.name)} className="h-14 w-full" />
                    </span>
                    <div className="min-w-0 flex-1">
                      {editing ? <input aria-label="Component name" value={component.name} onChange={(event) => updateComponent(index, { name: event.target.value })} className={fieldClass} /> : <p className="truncate text-sm font-semibold text-foreground">{component.name}</p>}
                      <div className="mt-1 flex flex-wrap items-center gap-2">
                        <span className="text-xs text-muted-foreground">{component.interface_type || "Interface not set"}</span>
                        <SourceBadges sources={component.sources} />
                      </div>
                    </div>
                    <div className="w-16 shrink-0 text-right" title={`${confidence}% confidence`}>
                      <p className="font-mono text-sm font-semibold text-foreground">{confidence}%</p>
                      <p className="font-mono text-[9px] tracking-wide text-subtle uppercase">confidence</p>
                      <span className="mt-1 block h-0.5 w-full overflow-hidden rounded-full bg-line-soft"><span className={cn("block h-full origin-left rounded-full", confidence >= 80 ? "bg-pass" : confidence >= 50 ? "bg-warn" : "bg-fail")} style={{ transform: `scaleX(${confidence / 100})` }} /></span>
                    </div>
                    {editing && <Button variant="ghost" size="icon" className="size-8 text-muted-foreground hover:text-fail" aria-label={`Remove ${component.name}`} onClick={() => setDraft({ ...draft, components: draft.components.filter((_, itemIndex) => itemIndex !== index) })}><Trash2 /></Button>}
                  </li>
                );
              })}
            </ul>
          )}
        </motion.section>

        {/* Connections */}
        <motion.section variants={reveal} aria-labelledby="connections-title" className="min-w-0 border-t border-border pt-10 xl:border-t-0 xl:border-l xl:pt-0 xl:pl-10">
          <SectionTitle eyebrow={`Pins & signals · ${connections.length}`} title="Connections" action={editing && (
            <Button variant="ghost" size="sm" onClick={() => { const next = connections.length + 1; setDraft({ ...draft, connections: [...connections, { id: `user-connection-${next}`, component_name: draft.components[0]?.name ?? "Component", role: "SIGNAL", target: "", direction: "unknown", behavior: "digital_input", expected: blankExpected("digital_input"), confidence: 1, sources: ["USER"], required: true, confirmed: false }] }); }}><Plus /> Add</Button>
          )} />
          {connections.length === 0 ? (
            <p className="mt-4 rounded-lg border border-dashed border-border px-4 py-5 text-sm text-muted-foreground">No signal connection found. Add one before confirming.</p>
          ) : editing ? (
            <div className="mt-4 space-y-2">
              {connections.map((connection, index) => {
                const key = behaviorKey(connection.behavior);
                const known = behaviorOptions.some(([value]) => value === key);
                return (
                  <div key={connection.id} className="rounded-lg bg-surface p-3 ring-1 ring-border">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <span className="font-mono text-xs font-medium text-foreground">{connection.role || "Unnamed"}</span>
                      <span className="flex items-center gap-2"><SourceBadges sources={connection.sources} />
                        <Button variant="ghost" size="icon" className="size-7 text-muted-foreground hover:text-fail" aria-label={`Remove ${connection.role}`} onClick={() => setDraft({ ...draft, connections: connections.filter((_, itemIndex) => itemIndex !== index) })}><Trash2 /></Button>
                      </span>
                    </div>
                    <div className="grid grid-cols-2 gap-2 sm:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)_4.5rem_minmax(0,1.1fr)]">
                      <label className="text-[11px] text-subtle">Component<input value={connection.component_name} onChange={(event) => updateConnection(index, { component_name: event.target.value })} className={cn(fieldClass, "mt-1")} /></label>
                      <label className="text-[11px] text-subtle">Role<input value={connection.role} onChange={(event) => updateConnection(index, { role: event.target.value })} className={cn(fieldClass, "mt-1 font-mono")} /></label>
                      <label className="text-[11px] text-subtle">GPIO<input type="number" min="0" max="99" value={connection.gpio ?? ""} onChange={(event) => updateConnection(index, { gpio: event.target.value === "" ? undefined : Number(event.target.value) })} className={cn(fieldClass, "mt-1 font-mono")} /></label>
                      <label className="text-[11px] text-subtle">Behavior
                        <select value={known ? key : connection.behavior} onChange={(event) => updateConnection(index, { behavior: event.target.value, expected: { ...connection.expected, signal_type: event.target.value.replaceAll("_", " ") } })} className={cn(fieldClass, "mt-1")}>
                          {!known && <option value={connection.behavior}>{connection.behavior || "Not set"}</option>}
                          {behaviorOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                        </select>
                      </label>
                      <label className="col-span-2 text-[11px] text-subtle sm:col-span-4">Target node<input value={connection.target} onChange={(event) => updateConnection(index, { target: event.target.value })} className={cn(fieldClass, "mt-1")} /></label>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="mt-4 overflow-x-auto">
              <table className="w-full min-w-[520px] text-left text-sm">
                <thead>
                  <tr className="border-b border-border font-mono text-[10px] tracking-[0.12em] text-subtle uppercase">
                    <th className="pb-2.5 pr-3 font-normal">Signal</th><th className="pb-2.5 pr-3 font-normal">Pin</th><th className="pb-2.5 pr-3 font-normal">Behavior</th><th className="pb-2.5 pr-3 font-normal">Part</th><th className="pb-2.5 font-normal">Source</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line-soft">
                  {connections.map((connection) => {
                    const family = behaviorFamily(connection.behavior);
                    return (
                      <tr key={connection.id} title={connection.target} className="transition-colors duration-150 hover:bg-surface/60">
                        <td className="py-3 pr-3"><span className="rounded-md bg-surface-2 px-2 py-1 font-mono text-xs font-bold text-foreground ring-1 ring-border">{connection.role}</span></td>
                        <td className="py-3 pr-3 font-mono text-sm font-semibold text-foreground">{connection.gpio != null ? <>GPIO<span className="text-signal">{connection.gpio}</span></> : <span className="font-normal text-subtle">—</span>}</td>
                        <td className="py-3 pr-3"><span className="inline-flex items-center gap-2 text-sm font-medium text-foreground"><span className={cn("size-2 rounded-full", family.dot)} />{behaviorLabel(connection.behavior)}</span></td>
                        <td className="max-w-[11rem] truncate py-3 pr-3 text-xs text-muted-foreground">{connection.component_name}</td>
                        <td className="py-3"><SourceBadges sources={connection.sources} /></td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 font-mono text-[10px] text-subtle">
                {[["Power", "bg-warn"], ["Digital", "bg-signal"], ["Pulse / PWM", "bg-pass"], ["Bus", "bg-muted-foreground"]].map(([label, dot]) => <span key={label} className="inline-flex items-center gap-1.5"><span className={cn("size-1.5 rounded-full", dot)} />{label}</span>)}
              </p>
            </div>
          )}
        </motion.section>
      </div>
    </motion.div>
  );
}
