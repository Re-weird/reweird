"use client";

import { memo, useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  Activity, ArrowRight, ArrowUpRight, Check, ChevronRight, Cpu, FileText, GitBranch, History, LockKeyhole, Radio,
  TriangleAlert, Waves,
} from "lucide-react";
import type { DemoSession, HistorySummary, ProbePlan, ProbeReading, Project, ProjectProfile, RuleResult, SimulatorScenario } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { historyApi } from "@/lib/api";
import type { ProjectTabID } from "@/lib/project-routes";
import { cn } from "@/lib/utils";
import { realBreakReady, telemetryLabel } from "@/lib/weird-demo";
import { JudgeCircuit } from "./judge-circuit";

type Props = {
  session: DemoSession;
  project: Project | null;
  profile: ProjectProfile | null;
  source: "api" | "browser";
  onNavigate: (tab: ProjectTabID) => void;
  /** Recent sessions are limited to this project's history. */
  historyProjectID: string;
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
          <div className="flex justify-between font-mono text-[10px] text-subtle">{ticks.map((tick) => <span key={tick}>{tick}s</span>)}</div>
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
    <motion.section variants={reveal} aria-labelledby="signal-monitor-title" className="min-w-0">
      <div className="flex flex-wrap items-end justify-between gap-5 border-b border-border pb-4">
        <div>
          <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">Observe</p>
          <h2 id="signal-monitor-title" className="mt-1 text-lg font-semibold tracking-tight text-foreground">Signal monitor</h2>
        </div>
        <div className="flex gap-7">
          <Stat icon={Activity} label="Active probes" value={live ? String(activeProbes.length) : "—"} />
          <Stat icon={Waves} label="Capture window" value={live && session.raw_telemetry ? `${session.raw_telemetry.window_ms / 1000}s` : "—"} />
          <Stat icon={Radio} label="Failed checks" value={live ? String(failures) : "—"} alert={live && failures > 0} />
        </div>
      </div>

      {live ? (
        <>
          <motion.div variants={{ show: { transition: { staggerChildren: 0.05 } } }} className="mt-5 grid grid-cols-2 gap-3 lg:grid-cols-3">
            {activeProbes.map((reading) => <ProbeTile key={reading.probe} reading={reading} />)}
          </motion.div>
          {idleProbes.length > 0 && <p className="mt-3 font-mono text-[11px] text-subtle">{idleProbes.map((probe) => probe.probe).join(", ")} not assigned</p>}
          <div className="mt-8 grid grid-cols-1 gap-8 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
            <ActivityTimeline probes={activeProbes.filter((probe) => probe.samples?.length)} windowMS={session.raw_telemetry?.window_ms ?? 60_000} />
            <div>
              <div className="flex items-baseline justify-between gap-2">
                <h3 className="text-sm font-semibold text-foreground">Rule checks</h3>
                <p className="flex gap-2.5 font-mono text-[11px]">{counts.map(([status, count]) => <span key={status} className={status === "fail" ? "text-fail" : status === "warn" ? "text-warn" : "text-pass"}>{count} {status}</span>)}</p>
              </div>
              <ul className="mt-2 divide-y divide-line-soft">{ordered.map((rule, index) => <RuleLine key={`${rule.probe ?? ""}-${rule.id}-${index}`} rule={rule} />)}</ul>
              <p className="mt-3 text-xs text-subtle">Deterministic checks run on raw readings before any AI interpretation.</p>
            </div>
          </div>
        </>
      ) : (
        <div className="mt-5">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-3" aria-hidden>
            {(plan?.instructions.filter((item) => item.probe !== "GND") ?? Array.from({ length: 6 }, (_, index) => ({ probe: `P${index + 1}`, role: "Unassigned" }))).slice(0, 6).map((slot) => (
              <div key={slot.probe} className="flex h-28 flex-col justify-between rounded-lg border border-dashed border-border p-4">
                <p className="font-mono text-[11px] text-subtle">{slot.probe}</p>
                <p className="truncate text-sm text-muted-foreground">{slot.role}</p>
              </div>
            ))}
          </div>
          <div className="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-lg bg-surface px-4 py-3 ring-1 ring-border">
            <p className="text-sm text-muted-foreground">No readings yet.{currentStep ? ` Next step: ${currentStep.title}.` : ""}</p>
            {currentStep?.action && <Button size="sm" variant="outline" onClick={() => onNavigate(currentStep.action!.tab)}>{currentStep.action.label} <ChevronRight /></Button>}
          </div>
        </div>
      )}
    </motion.section>
  );
}

function LatestAnalysis({ session, live, onNavigate }: { session: DemoSession; live: boolean; onNavigate: (tab: ProjectTabID) => void }) {
  const failures = session.evidence.rule_results.filter((rule) => rule.status === "fail").length;
  return (
    <motion.section variants={reveal} aria-labelledby="analysis-title" className="border-b border-border pb-7">
      <p className="font-mono text-[11px] tracking-[0.14em] text-subtle uppercase">Understand</p>
      <h2 id="analysis-title" className="mt-1 text-lg font-semibold tracking-tight text-foreground">Latest analysis</h2>
      {live ? (
        <>
          <div className="mt-4 flex items-start gap-3">
            <span className={cn("mt-0.5 grid size-8 shrink-0 place-items-center rounded-md ring-1", failures ? "text-fail ring-fail/40" : "text-pass ring-border")}><Activity className="size-4" strokeWidth={1.5} /></span>
            <div>
              <p className="text-base font-semibold leading-snug text-foreground">{session.diagnosis.headline}</p>
              <p className="mt-1 font-mono text-xs text-muted-foreground">{Math.round(session.diagnosis.confidence * 100)}% confidence · interpretation</p>
            </div>
          </div>
          <p className="mt-3 max-w-[65ch] text-sm leading-relaxed text-muted-foreground">{session.diagnosis.summary}</p>
          {session.diagnosis.possible_causes.length > 0 && (
            <>
              <p className="mt-5 flex items-baseline justify-between text-xs"><span className="font-semibold text-foreground">Possible causes</span><span className="text-subtle">Not yet confirmed</span></p>
              <ol className="mt-2 space-y-2">
                {session.diagnosis.possible_causes.slice(0, 3).map((cause, index) => (
                  <li key={cause} className="flex gap-3 text-sm text-muted-foreground"><span className="font-mono text-xs text-subtle">{String(index + 1).padStart(2, "0")}</span><span>{cause}</span></li>
                ))}
              </ol>
            </>
          )}
          <div className="mt-5 rounded-lg bg-surface p-4 ring-1 ring-border">
            <p className="text-xs font-semibold text-foreground">Next diagnostic test</p>
            <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{session.diagnosis.next_test}</p>
            <div className="mt-3 flex flex-wrap gap-2">
              <Button size="sm" className="active:scale-[0.98]" onClick={() => onNavigate("next-test")}>Open test planner <ArrowUpRight /></Button>
              <Button size="sm" variant="outline" onClick={() => onNavigate("diagnosis")}>Review evidence</Button>
            </div>
          </div>
        </>
      ) : (
        <p className="mt-3 text-sm leading-relaxed text-muted-foreground">A diagnosis appears here after the first capture. Rule checks run on the raw readings first; any interpretation is labeled as one.</p>
      )}
    </motion.section>
  );
}

function RecentSessions({ projectID, onNavigate }: { projectID: string; onNavigate: (tab: ProjectTabID) => void }) {
  const [items, setItems] = useState<HistorySummary[] | null>(null);
  const [offline, setOffline] = useState(false);
  useEffect(() => {
    let active = true;
    setItems(null); setOffline(false);
    historyApi.list({ projectID }).then(({ items: list }) => { if (active) setItems(list.slice(0, 3)); }).catch(() => { if (active) setOffline(true); });
    return () => { active = false; };
  }, [projectID]);
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

export function Workbench({ session, project, profile, source, onNavigate, historyProjectID, plan }: Props) {
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

  return (
    <div>
      <motion.div data-tw initial={reduce ? false : "hidden"} animate="show" variants={{ show: { transition: { staggerChildren: 0.07 } } }}>
        <ProjectHeader project={project} profile={profile} name={name} live={live} stepIndex={Math.max(currentIndex, 0)} source={source} session={session} />
        <AnimatePresence initial={false}>
          {!live && <SetupChecklist key="setup" steps={steps} onNavigate={onNavigate} />}
        </AnimatePresence>
      </motion.div>

      {canBreakPhysical && <div className="mt-8"><JudgeCircuit key={`${profile?.id}-${session.raw_telemetry?.device_id}`} session={session} onDiagnose={() => onNavigate("diagnosis")} onTest={() => onNavigate("next-test")} onVerify={() => onNavigate("verify")} /></div>}

      <motion.div data-tw initial={reduce ? false : "hidden"} animate="show" variants={{ show: { transition: { staggerChildren: 0.08, delayChildren: 0.1 } } }}
        className="mt-8 grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-12">
        <SignalMonitor session={session} live={live} plan={plan} failures={failures} currentStep={steps[currentIndex]} onNavigate={onNavigate} />
        <aside className="min-w-0">
          <LatestAnalysis session={session} live={live} onNavigate={onNavigate} />
          <RecentSessions projectID={historyProjectID} onNavigate={onNavigate} />
          <motion.p variants={reveal} className="flex items-center gap-2 border-t border-border pt-5 text-xs text-subtle">
            <LockKeyhole className="size-3.5" strokeWidth={1.5} /> Passive sensing. Electrical output stays locked.
          </motion.p>
        </aside>
      </motion.div>
    </div>
  );
}
