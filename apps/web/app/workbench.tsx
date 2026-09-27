"use client";

import { memo, useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  Bolt, Activity, ArrowRight, ArrowUpRight, Check, ChevronRight, Cpu, FileText, GitBranch, History, LockKeyhole, Radio,
  TriangleAlert, Waves,
} from "lucide-react";
import type { DemoSession, HistorySummary, ProbePlan, ProbeReading, Project, ProjectProfile, RuleResult, SimulatorScenario } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { historyApi } from "@/lib/api";
import type { ProjectTabID } from "@/lib/project-routes";
import { cn } from "@/lib/utils";
import { realBreakReady, telemetryLabel } from "@/lib/weird-demo";
import { CalibrationPanel } from "./calibration-panel";
import { JudgeCircuit } from "./judge-circuit";
import { ProbeCard, RuleRow, SignalChart } from "./signal-components";

type Props = {
  session: DemoSession;
  project: Project | null;
  profile: ProjectProfile | null;
  source: "api" | "browser";
  onNavigate: (tab: ProjectTabID) => void;
  /** Recent sessions are limited to this project's history. */
  historyProjectID: string;
  demoHistory?: HistorySummary[];
  plan: ProbePlan | null;
  scenarios: SimulatorScenario[];
  busy: boolean;
  onRunScenario: (scenarioID: string, mystery: boolean) => Promise<void>;
  onBrowserDemo: () => Promise<void>;
};

const EASE = [0.32, 0.72, 0, 1] as const;
const reveal = {
  hidden: { opacity: 0, y: 14 },
  show: { opacity: 1, y: 0, transition: { duration: 0.6, ease: EASE } },
};

type StepState = "done" | "current" | "upcoming";
type SetupStep = { id: string; title: string; detail: string; state: StepState; action?: { label: string; tab: ProjectTabID } };

function setupSteps(project: Project | null, profile: ProjectProfile | null, plan: ProbePlan | null, capturing: boolean): SetupStep[] {
  const repo = project?.repository;
  const analyzed = Boolean(profile);
  const raw: Omit<SetupStep, "state">[] = [
    {
      id: "code",
      title: "Code analyzed",
      detail: analyzed
        ? repo?.last_commit ? `${repo.full_name} @ ${repo.last_commit.sha.slice(0, 7)}` : "Draft profile generated"
        : repo ? repo.sync_error ?? `Waiting for ${repo.default_branch} to be read` : "No repository linked to this project",
      action: { label: repo ? "Open overview" : "Project overview", tab: "overview" },
    },
    { id: "profile", title: "Profile confirmed", detail: profile?.confirmed ? `${profile.components.length} components, ${profile.connections?.length ?? 0} connections` : "Review components and pin roles, then confirm", action: { label: "Review profile", tab: "overview" } },
    { id: "probes", title: "Probes connected", detail: plan?.connected ? `${plan.instructions.length} probe points wired` : plan ? "Wire GND and P1–P6 as planned" : "Plan appears after the profile is confirmed", action: { label: "Open probe setup", tab: "probe-setup" } },
    { id: "capture", title: "First capture", detail: capturing ? "Readings are arriving" : "Waiting for readings from your bench device" },
  ];
  const done = [analyzed, Boolean(profile?.confirmed), Boolean(plan?.connected), capturing];
  const current = done.indexOf(false);
  return raw.map((step, index) => ({ ...step, state: done[index] ? "done" : index === current ? "current" : "upcoming" }));
}

// Isolated so its infinite pulse never re-renders the page.
const PulseDot = memo(function PulseDot({ tone }: { tone: "live" | "idle" }) {
  const reduce = useReducedMotion();
  const color = tone === "live" ? "bg-pass" : "bg-signal";
  return (
    <span className="relative inline-flex size-2">
      {!reduce && <motion.span className={cn("absolute inset-0 rounded-full", color)} animate={{ scale: [1, 2.4], opacity: [0.5, 0] }} transition={{ duration: 1.8, repeat: Infinity, ease: "easeOut" }} />}
      <span className={cn("relative inline-flex size-2 rounded-full", color)} />
    </span>
  );
});

function ProjectHeader({ project, profile, name, live, stepIndex, source, session }: { project: Project | null; profile: ProjectProfile | null; name: string; live: boolean; stepIndex: number; source: "api" | "browser"; session: DemoSession }) {
  const repo = project?.repository;
  const controller = profile?.controller ?? project?.controller ?? "Controller unspecified";
  const voltage = profile?.logic_voltage ?? project?.logic_voltage;
  return (
    <motion.header variants={reveal} className="flex flex-col gap-5 border-b border-border pb-7 md:flex-row md:items-end md:justify-between">
      <div className="min-w-0">
        <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">{project ? "Workbench" : "Built-in demo · workbench"}</p>
        <h1 className="mt-2 truncate text-3xl font-semibold tracking-tight text-foreground md:text-4xl">{name}</h1>
        <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted-foreground">
          {repo && (
            <a href={repo.html_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 hover:text-foreground">
              <GitBranch className="size-3.5" strokeWidth={1.5} />{repo.full_name}
              {repo.last_commit && <span className="font-mono text-xs text-subtle">@{repo.last_commit.sha.slice(0, 7)}</span>}
            </a>
          )}
          <span className="inline-flex items-center gap-1.5"><Cpu className="size-3.5" strokeWidth={1.5} />{controller}{voltage ? <span className="font-mono text-xs">· {voltage} V</span> : null}</span>
          {profile && <span>{profile.components.length} component{profile.components.length === 1 ? "" : "s"}</span>}
          <span className={cn("inline-flex items-center gap-1.5", profile?.confirmed ? "text-pass" : "text-warn")}>
            {profile?.confirmed ? <Check className="size-3.5" strokeWidth={2} /> : <TriangleAlert className="size-3.5" strokeWidth={1.5} />}
            {profile?.confirmed ? "Profile confirmed" : profile ? "Profile needs review" : "No profile yet"}
          </span>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2.5 rounded-full bg-surface px-3.5 py-1.5 text-xs ring-1 ring-border">
        <PulseDot tone={live ? "live" : "idle"} />
        <span className="font-medium text-foreground">{live ? "Live" : `Setting up · step ${stepIndex + 1} of 4`}</span>
        {live && <span className="font-mono text-[10px] text-subtle">{telemetryLabel(session, source)}</span>}
      </div>
    </motion.header>
  );
}

function SetupChecklist({ steps, onNavigate }: { steps: SetupStep[]; onNavigate: (tab: ProjectTabID) => void }) {
  return (
    <motion.section layout variants={reveal} exit={{ opacity: 0, y: -8 }} aria-label="Project setup" className="py-8">
      <h2 className="text-sm font-semibold text-foreground">Finish setting up</h2>
      <p className="mt-1 text-sm text-muted-foreground">Live diagnostics start once your bench device sends its first reading.</p>
      <ol className="relative mt-6 grid grid-cols-1 gap-6 md:grid-cols-4 md:gap-4">
        {steps.map((step, index) => (
          <li key={step.id} className="relative flex gap-4 md:flex-col md:gap-3">
            {index < steps.length - 1 && <span aria-hidden className={cn("absolute top-4 left-10 -right-4 hidden h-px md:block", step.state === "done" ? "bg-pass/50" : "bg-border")} />}
            <span className={cn(
              "relative z-1 grid size-8 shrink-0 place-items-center rounded-full font-mono text-xs ring-1 transition-colors",
              step.state === "done" && "bg-surface-2 text-pass ring-border",
              step.state === "current" && "bg-background text-foreground ring-signal",
              step.state === "upcoming" && "bg-background text-subtle ring-border",
            )}>
              {step.state === "done" ? <Check className="size-4" strokeWidth={2} /> : String(index + 1).padStart(2, "0")}
            </span>
            <div className="min-w-0">
              <p className={cn("text-sm font-semibold", step.state === "upcoming" ? "text-muted-foreground" : "text-foreground")}>{step.title}</p>
              <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{step.detail}</p>
              {step.state === "current" && step.action && (
                <Button size="sm" className="mt-3 active:scale-[0.98]" onClick={() => onNavigate(step.action!.tab)}>
                  {step.action.label} <ArrowRight />
                </Button>
              )}
            </div>
          </li>
        ))}
      </ol>
    </motion.section>
  );
}

const probeTone: Record<ProbeReading["status"], string> = { stable: "text-pass", active: "text-signal", intermittent: "text-fail", idle: "text-subtle" };

function ProbeTile({ reading }: { reading: ProbeReading }) {
  const alert = reading.status === "intermittent";
  return (
    <motion.article layout variants={reveal} className={cn("flex flex-col gap-4 rounded-lg bg-surface p-4 ring-1", alert ? "ring-fail/50" : "ring-border")}>
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="font-mono text-[11px] text-subtle">{reading.probe}</p>
          <p className="truncate text-sm font-medium text-foreground">{reading.role}</p>
        </div>
        <span className={cn("font-mono text-[10px] uppercase", probeTone[reading.status])}>{reading.status}</span>
      </div>
      <p className="font-mono text-2xl tracking-tight text-foreground">
        {reading.value === null ? "—" : reading.value.toFixed(reading.unit === "V" ? 2 : 1)}
        <span className="ml-1 text-xs text-muted-foreground">{reading.unit}</span>
      </p>
      <p className={cn("font-mono text-[11px]", reading.dropouts > 0 ? "text-fail" : "text-subtle")}>{reading.dropouts > 0 ? `${reading.dropouts} dropout events` : "No dropouts"}</p>
    </motion.article>
  );
}

// Samples are per-slice activity for the capture window (e.g. 12 × 5 s over
// 60 s); a zero slice means the probe saw nothing then. Analog probes only
// report presence, so their slices read as present/absent, not volts.
function sliceLabel(reading: ProbeReading, value: number) {
  if (value === 0) return "no activity";
  if (reading.unit === "V") return "signal present";
  return `${Number.isInteger(value) ? value : value.toFixed(1)} ${reading.unit}`;
}

function ActivityTimeline({ probes, windowMS }: { probes: ProbeReading[]; windowMS: number }) {
  const [hover, setHover] = useState<{ row: number; col: number } | null>(null);
  const slices = Math.max(...probes.map((probe) => probe.samples?.length ?? 0), 0);
  if (!slices) return null;
  const sliceSeconds = windowMS / 1000 / slices;
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((fraction) => Math.round((windowMS / 1000) * fraction));
  const hovered = hover ? probes[hover.row] : null;
  const hoveredValue = hovered?.samples?.[hover!.col];
  return (
    <div className="min-w-0">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-sm font-semibold text-foreground">Activity over the capture</h3>
        <p className="font-mono text-[11px] text-subtle">{slices} × {sliceSeconds} s slices</p>
      </div>
      <div className="mt-4 space-y-2.5" onMouseLeave={() => setHover(null)}>
        {probes.map((probe, row) => {
          const values = probe.samples ?? [];
          const peak = Math.max(...values, 0);
          const gaps = values.filter((value) => value === 0).length;
          return (
            <div key={probe.probe} className="grid grid-cols-[88px_minmax(0,1fr)_64px] items-center gap-3">
              <p className="truncate font-mono text-[11px] text-muted-foreground"><span className="text-subtle">{probe.probe}</span> {probe.role}</p>
              <div className="grid gap-[3px]" style={{ gridTemplateColumns: `repeat(${slices}, minmax(0, 1fr))` }} role="img"
                aria-label={`${probe.probe} ${probe.role}: ${gaps ? `${gaps} of ${values.length} slices with no activity` : "activity in every slice"}`}>
                {values.map((value, col) => {
                  const active = hover?.col === col;
                  return (
                    <span key={col} onMouseEnter={() => setHover({ row, col })}
                      className={cn("h-7 rounded-[3px] transition-shadow", active && "ring-1 ring-foreground/50", value === 0 && "ring-1 ring-fail/60")}
                      style={value === 0
                        ? { backgroundImage: "repeating-linear-gradient(135deg, color-mix(in srgb, var(--red) 28%, transparent) 0 2px, transparent 2px 6px)" }
                        : { backgroundColor: "var(--cyan)", opacity: 0.2 + 0.45 * (peak ? value / peak : 1) }} />
                  );
                })}
              </div>
              <p className={cn("text-right font-mono text-[11px]", gaps ? "text-fail" : "text-subtle")}>{gaps ? `${gaps} empty` : "steady"}</p>
            </div>
          );
        })}
        <div className="grid grid-cols-[88px_minmax(0,1fr)_64px] gap-3">
          <span />
          <div className="flex justify-between font-mono text-[10px] text-subtle">{ticks.map((tick, position) => <span key={`tick-${position}`}>{tick}s</span>)}</div>
          <span />
        </div>
      </div>
      <p className="mt-3 h-4 font-mono text-[11px] text-muted-foreground" aria-live="polite">
        {hovered && hoveredValue !== undefined
          ? `${hovered.probe} ${hovered.role} · ${hover!.col * sliceSeconds}–${(hover!.col + 1) * sliceSeconds} s · ${sliceLabel(hovered, hoveredValue)}`
          : "Hover a slice for its reading. Hatched slices had no activity at all."}
      </p>
    </div>
  );
}

function RuleLine({ rule }: { rule: RuleResult }) {
  const tone = rule.status === "pass" ? "text-pass" : rule.status === "warn" ? "text-warn" : "text-fail";
  return (
    <li className="flex items-start gap-3 py-2.5 text-sm">
      <span className={cn("mt-0.5 w-8 shrink-0 font-mono text-[10px] uppercase", tone)}>{rule.status}</span>
      <span className="text-foreground">{rule.message}</span>
    </li>
  );
}

function Stat({ icon: Icon, label, value, alert = false }: { icon: typeof Activity; label: string; value: string; alert?: boolean }) {
  return (
    <div className="flex items-center gap-2.5">
      <Icon className={cn("size-4", alert ? "text-fail" : "text-subtle")} strokeWidth={1.5} />
      <div>
        <p className={cn("font-mono text-base leading-none", alert ? "text-fail" : "text-foreground")}>{value}</p>
        <p className="mt-1 text-[11px] text-muted-foreground">{label}</p>
      </div>
    </div>
  );
}

function SignalMonitor({ session, live, plan, failures, currentStep, onNavigate }: { session: DemoSession; live: boolean; plan: ProbePlan | null; failures: number; currentStep: SetupStep | undefined; onNavigate: (tab: ProjectTabID) => void }) {
  const activeProbes = session.probes.filter((probe) => probe.status !== "idle");
  const idleProbes = session.probes.filter((probe) => probe.status === "idle");
  const rules = session.evidence.rule_results;
  const counts = (["fail", "warn", "pass"] as const).map((status) => [status, rules.filter((rule) => rule.status === status).length] as const).filter(([, count]) => count > 0);
  const ordered = [...rules].sort((a, b) => ["fail", "warn", "pass"].indexOf(a.status) - ["fail", "warn", "pass"].indexOf(b.status));
  return (
    <section className="bench-panel bench-signals" aria-labelledby="signal-monitor-title">
      <div className="bench-panel-head"><div><span className="bench-label">01 / Observe</span><h2 id="signal-monitor-title">Signal monitor</h2></div></div>
      {live ? <>
        <div className="probe-grid">{session.probes.map((reading) => <ProbeCard key={reading.probe} reading={reading} />)}</div>
        <div className="two-column wide-left">
          <div className="panel">
            <div className="panel-heading"><div><span className="eyebrow">Raw activity buckets</span><h2>Signal activity</h2></div>
              <div className="chart-legend">{session.probes.filter((probe) => probe.samples?.length).slice(0, 2).map((probe) => <span key={probe.probe}>{probe.probe} {probe.role}</span>)}</div>
            </div>
            <SignalChart session={session} windowMS={session.raw_telemetry?.window_ms ?? 60_000} />
          </div>
          <div className="panel">
            <div className="panel-heading"><div><span className="eyebrow">Analysis</span><h2>Rule engine</h2></div></div>
            <div className="rules-list">{ordered.map((rule, index) => <RuleRow key={`${rule.probe ?? ""}-${rule.id}-${index}`} rule={rule} />)}</div>
            <div className="engine-note"><Bolt size={16} /><span>Deterministic checks run before any AI interpretation.</span></div>
          </div>
        </div>
      </> : <div className="bench-empty"><Radio size={24} /><h3>Waiting for this project’s signals</h3>
        <p>No readings yet.{currentStep ? ` Next step: ${currentStep.title}.` : ""}</p>
        {currentStep?.action && <button className="secondary" onClick={() => onNavigate(currentStep.action!.tab)}>{currentStep.action.label} <ChevronRight size={15} /></button>}
      </div>}
    </section>
  );
}

function LatestAnalysis({ session, live, source, onNavigate }: { session: DemoSession; live: boolean; source: "api" | "browser"; onNavigate: (tab: ProjectTabID) => void }) {
  const failures = session.evidence.rule_results.filter((rule) => rule.status === "fail").length;
  return (
    <section className="bench-panel bench-analysis" aria-labelledby="analysis-title">
      <div className="bench-panel-head"><div><span className="bench-label">02 / Understand</span><h2 id="analysis-title">Latest analysis</h2></div><span className="analysis-source">{!live ? "Pending" : telemetryLabel(session, source)}</span></div>
      {live ? <>
        {failures > 0 && <div className="weird-analysis-kicker">SOMETHING’S WEIRD. <small>{session.evidence.probe} · {session.evidence.role} · {failures} failed {failures === 1 ? "check" : "checks"}</small></div>}
        <div className="analysis-headline"><span className={`analysis-mark ${failures ? "attention" : ""}`}><Activity size={21} /></span><div><h3>{session.diagnosis.headline}</h3><span className="analysis-confidence">{Math.round(session.diagnosis.confidence * 100)}% confidence <span>· interpretation</span></span></div></div>
        <p className="analysis-summary">{session.diagnosis.summary}</p>
        {session.diagnosis.possible_causes.length > 0 && <>
          <div className="hypothesis-heading"><span className="bench-label">Possible causes</span><small>Not yet confirmed</small></div>
          <ol className="bench-hypotheses">{session.diagnosis.possible_causes.slice(0, 3).map((cause, index) => <li key={`cause-${index}`}><span>{String(index + 1).padStart(2, "0")}</span><p>{cause}</p><ChevronRight size={13} /></li>)}</ol>
        </>}
        <div className="bench-next-test"><span className="bench-label">Next diagnostic test</span><p>{session.diagnosis.next_test}</p><button className="text-button" onClick={() => onNavigate("next-test")}>Open test planner <ArrowUpRight size={15} /></button></div>
        <div className="bench-analysis-actions"><button className="secondary" onClick={() => onNavigate("diagnosis")}>Review evidence</button><button className="primary" onClick={() => onNavigate("simulator")}>Simulator <ArrowUpRight size={15} /></button></div>
      </> : <div className="bench-empty"><Activity size={24} /><h3>Ready when your project is.</h3><p>A diagnosis appears after a matching capture. Rule checks run before interpretation.</p></div>}
    </section>
  );
}

function RecentSessions({ projectID, onNavigate, demoItems }: { projectID: string; onNavigate: (tab: ProjectTabID) => void; demoItems?: HistorySummary[] }) {
  const [items, setItems] = useState<HistorySummary[] | null>(null);
  const [offline, setOffline] = useState(false);
  useEffect(() => {
    if (demoItems) { setItems(demoItems.slice(0, 3)); setOffline(false); return; }
    let active = true;
    setItems(null); setOffline(false);
    historyApi.list({ projectID }).then(({ items: list }) => { if (active) setItems(list.slice(0, 3)); }).catch(() => { if (active) setOffline(true); });
    return () => { active = false; };
  }, [projectID, demoItems]);
  return (
    <motion.section variants={reveal} aria-labelledby="recent-title" className="py-7">
      <div className="flex items-center justify-between">
        <h2 id="recent-title" className="text-sm font-semibold text-foreground">Recent sessions</h2>
        <button type="button" onClick={() => onNavigate("history")} className="text-xs text-muted-foreground hover:text-foreground">View all</button>
      </div>
      {offline ? (
        <p className="mt-3 text-sm text-muted-foreground">Connect the API to browse saved sessions.</p>
      ) : !items ? (
        <div className="mt-3 space-y-2"><Skeleton className="h-10 bg-surface-2" /><Skeleton className="h-10 bg-surface-2" /></div>
      ) : items.length === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">Your first guided test starts this project&apos;s history.</p>
      ) : (
        <ul className="mt-2 divide-y divide-line-soft">
          {items.map((item) => (
            <li key={item.id}>
              <button type="button" onClick={() => onNavigate("history")} className="flex w-full items-center gap-3 py-2.5 text-left">
                <History className="size-4 shrink-0 text-subtle" strokeWidth={1.5} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm text-foreground">{item.original_problem || item.project_name}</span>
                  <span className="font-mono text-[11px] text-muted-foreground">{item.status.replaceAll("_", " ").toLowerCase()} · {new Date(item.started_at_ms).toLocaleDateString()}</span>
                </span>
                <ChevronRight className="size-4 text-subtle" strokeWidth={1.5} />
              </button>
            </li>
          ))}
        </ul>
      )}
      <button type="button" onClick={() => onNavigate("reports")} className="mt-4 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground">
        <FileText className="size-4" strokeWidth={1.5} /> Diagnostic reports
      </button>
    </motion.section>
  );
}

export function Workbench({ session, project, profile, source, onNavigate, historyProjectID, plan, demoHistory }: Props) {
  const reduce = useReducedMotion();
  // Live data belongs to this project only when the session is running its profile.
  const matchesProject = !project || (Boolean(profile) && session.profile_id === profile?.id);
  const capturing = matchesProject && Boolean(session.raw_telemetry ?? session.probes.some((probe) => probe.status !== "idle"));
  const steps = setupSteps(project, profile, plan, capturing);
  const live = !project || steps.every((step) => step.state === "done");
  const currentIndex = steps.findIndex((step) => step.state === "current");
  const failures = session.evidence.rule_results.filter((rule) => rule.status === "fail").length;
  const name = project?.name ?? profile?.project_name ?? session.project_name;
  const canBreakPhysical = realBreakReady(session, source, profile, plan);

  const activeProbes = session.probes.filter((probe) => probe.status !== "idle");
  const destinations: ProjectTabID[] = ["workbench", "diagnosis", "next-test", "verify"];
  const currentStep = { detect: 0, diagnose: 1, test: 2, repair: 2, verify: 3 }[session.stage];
  return <div className="workbench">
    <div className="workbench-main">
      <section className="bench-welcome">
        <div className="welcome-copy"><span className="bench-label"></span><h1>Welcome to your<br />workbench.</h1><p>Understand your project.<br />Follow the evidence. Find your next move.</p><span className="welcome-foot"></span></div>
        <div className="welcome-photo" aria-hidden="true" /><span className="welcome-caption" aria-hidden="true"></span>
      </section>
      <div data-tw><AnimatePresence initial={false}>{!live && <SetupChecklist key="setup" steps={steps} onNavigate={onNavigate} />}</AnimatePresence></div>
      {canBreakPhysical && <JudgeCircuit key={`${profile?.id}-${session.raw_telemetry?.device_id}`} session={session} onDiagnose={() => onNavigate("diagnosis")} onTest={() => onNavigate("next-test")} onVerify={() => onNavigate("verify")} />}
      <section className="bench-stats" aria-label="Current project summary">
        <div><Activity size={17} /><strong>{live ? activeProbes.length : "—"}<small>Active probes</small></strong></div>
        <div><Waves size={17} /><strong>{live && session.raw_telemetry ? `${session.raw_telemetry.window_ms / 1000}s` : "—"}<small>Capture window</small></strong></div>
        <div className={live && failures ? "stat-attention" : ""}><Radio size={17} /><strong>{live ? failures : "—"}<small>Failed checks</small></strong></div>
        <div><LockKeyhole size={17} /><strong>Locked<small>PATCH output</small></strong></div>
      </section>
      <SignalMonitor session={session} live={live} plan={plan} failures={failures} currentStep={steps[currentIndex]} onNavigate={onNavigate} />
      {profile?.confirmed && matchesProject && source === "api" && session.telemetry_mode === "serial" && <CalibrationPanel profileID={profile.id} compact />}
      <section className="bench-panel bench-flow">
        <div className="bench-panel-head"><div><span className="bench-label">The diagnostic process</span><h2>Every step, backed by evidence.</h2></div></div>
        <div className="bench-steps">{session.timeline.map((step, index) => <button key={step.id} className={live && index === currentStep ? "current" : ""} onClick={() => onNavigate(live ? destinations[index] ?? "workbench" : "probe-setup")}><span className="bench-step-number">{live && step.complete ? <Check size={14} /> : `0${index + 1}`}</span><strong>{step.label}</strong><small>{live ? step.detail : "Awaiting project capture"}</small></button>)}</div>
      </section>
    </div>
    <aside className="workbench-rail">
      <LatestAnalysis session={session} live={live} source={source} onNavigate={onNavigate} />
      <section className="bench-panel bench-project">
        <div className="bench-panel-head"><h2>On your bench</h2><span className="bench-label">{project ? "Project" : "Built-in demo"}</span></div>
        <button className="bench-project-link" onClick={() => onNavigate("overview")}><span className="project-thumbnail"><Cpu size={28} strokeWidth={1.25} /></span><span><strong>{name}</strong><small>{profile?.controller ?? project?.controller ?? "Controller unspecified"} · {profile?.logic_voltage ?? project?.logic_voltage ?? "—"} V logic</small></span><ChevronRight size={16} /></button>
        <div className="bench-project-meta"><span>{profile?.components.length ?? 0} components</span><span>{profile?.confirmed ? "Profile confirmed" : "Review profile"}</span></div>
        {project?.repository && <a className="bench-report-link" href={project.repository.html_url} target="_blank" rel="noreferrer"><GitBranch size={15} /><span>{project.repository.full_name}{project.repository.last_commit ? ` @${project.repository.last_commit.sha.slice(0, 7)}` : ""}</span><ArrowUpRight size={14} /></a>}
      </section>
      <section className="bench-panel bench-history"><div data-tw><RecentSessions projectID={historyProjectID} onNavigate={onNavigate} demoItems={demoHistory} /></div></section>
      <p className="bench-safety"><LockKeyhole size={13} /> Passive sensing. Electrical output stays locked.</p>
    </aside>
  </div>;
}
