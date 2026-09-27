"use client";

import { memo, useMemo } from "react";
import Link from "next/link";
import { motion, useReducedMotion, type Variants } from "framer-motion";
import { ArrowUpRight, FolderGit2, GitBranch, RefreshCw, WifiOff } from "lucide-react";
import type { HistorySummary, Project } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { pipelineStage, useAccountActivity, type PipelineStage } from "@/lib/account-activity";
import { cn } from "@/lib/utils";

// Every figure comes from the API: the caller's projects and the saved
// diagnostic sessions on them. Nothing is estimated or invented
// (design/design.md: no fabricated metrics).

const WEEKS = 52;
const DAY_MS = 86_400_000;
const spring = { type: "spring", stiffness: 100, damping: 20 } as const;

const stagger: Variants = { hidden: {}, show: { transition: { staggerChildren: 0.07, delayChildren: 0.05 } } };
const rise: Variants = { hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0, transition: spring } };


const outcomes = [
  { status: "RESOLVED", label: "Resolved", tone: "bg-pass" },
  { status: "IMPROVED", label: "Improved", tone: "bg-warn" },
  { status: "UNRESOLVED", label: "Unresolved", tone: "bg-fail" },
] as const;
const OPEN = new Set(["OPEN", "TESTING", "WAITING_FOR_USER", "VERIFYING"]);

const stages: { id: PipelineStage; label: string; hint: string }[] = [
  { id: "needs-code", label: "Waiting for code analysis", hint: "Repo not read yet, or analysis failed" },
  { id: "needs-review", label: "Profile to review", hint: "Draft ready; confirm it on Overview" },
  { id: "needs-probes", label: "Probes to connect", hint: "Profile confirmed; wire the probe plan" },
  { id: "ready", label: "Ready for live diagnosis", hint: "Probes connected" },
];

function dayKey(ms: number) {
  const date = new Date(ms);
  date.setHours(0, 0, 0, 0);
  return date.getTime();
}

function ago(ms: number) {
  const minutes = Math.max(0, Math.floor((Date.now() - ms) / 60_000));
  if (minutes < 60) return minutes < 1 ? "just now" : `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return days < 30 ? `${days}d ago` : new Date(ms).toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function useCalendar() {
  return useMemo(() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const start = new Date(today.getTime() - (today.getDay() + (WEEKS - 1) * 7) * DAY_MS);
    const weeks = Array.from({ length: WEEKS }, (_, w) => new Date(start.getTime() + w * 7 * DAY_MS));
    // One label per month, starting at the week the month begins. A month that
    // gets fewer than 3 week columns before the next label is skipped (as
    // GitHub does), otherwise e.g. a partial "Sep" collides with "Oct".
    const starts = weeks.flatMap((week, index) => {
      const previous = weeks[index - 1];
      return !previous || previous.getMonth() !== week.getMonth() ? [{ start: index, label: week.toLocaleString("en", { month: "short" }) }] : [];
    });
    const months = starts
      .map((month, index) => ({ ...month, span: (starts[index + 1]?.start ?? WEEKS) - month.start }))
      .filter((month) => month.span >= 3);
    return { weeks, months };
  }, []);
}

function SectionHead({ title, meta }: { title: string; meta?: string }) {
  return (
    <div className="mb-5 flex items-baseline justify-between gap-4">
      <h2 className="text-[13px] font-semibold tracking-tight text-foreground">{title}</h2>
      {meta && <span className="font-mono text-[11px] text-subtle">{meta}</span>}
    </div>
  );
}


const ScanSweep = memo(function ScanSweep() {
  const reduce = useReducedMotion();
  if (reduce) return null;
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
      <motion.div
        className="h-full w-1/6 bg-linear-to-r from-transparent via-signal/12 to-transparent"
        initial={{ x: "-100%" }}
        animate={{ x: "600%" }}
        transition={{ duration: 7, repeat: Infinity, ease: "easeInOut", repeatDelay: 1.5 }}
      />
    </div>
  );
});

// A full year that stretches to the row width: one fluid CSS grid, week
// columns share the width (minmax keeps cells legible and scrolls on small
// screens). Cells fade in column by column via a CSS animation - 364 motion
// components would be needlessly heavy.
// Heat steps are relative to the busiest day, so one busy day doesn't need a
// fixed scale; the title carries the absolute count.
const heatTones = ["bg-line-soft", "bg-signal/25", "bg-signal/50", "bg-signal/75", "bg-signal"];
function heatTone(count: number, peak: number) {
  if (!count) return heatTones[0];
  return heatTones[Math.min(4, Math.max(1, Math.ceil((count / peak) * 4)))];
}

function ActivityHeatmap({ counts }: { counts: Map<number, { sessions: number; projects: number }> }) {
  const { weeks, months } = useCalendar();
  const peak = Math.max(1, ...Array.from(counts.values(), (value) => value.sessions + value.projects));
  const columns = `1.75rem repeat(${WEEKS}, minmax(9px, 1fr))`;
  return (
    <div className="relative w-full">
      <div className="relative grid gap-[3px] font-mono text-[10px] text-subtle" style={{ gridTemplateColumns: columns }}>
        <span style={{ gridColumn: 1, gridRow: 1 }} />
        {months.map((month) => (
          <span key={month.start} className="truncate" style={{ gridColumn: `${month.start + 2} / span ${month.span}`, gridRow: 1 }}>{month.label}</span>
        ))}
        {Array.from({ length: 7 }, (_, day) => (
          <div key={day} className="contents">
            <span className="flex items-center leading-none" style={{ gridColumn: 1, gridRow: day + 2 }}>{day % 2 === 1 ? ["", "Mon", "", "Wed", "", "Fri", ""][day] : ""}</span>
            {weeks.map((week, column) => {
              const date = week.getTime() + day * DAY_MS;
              if (date > Date.now()) return <span key={week.getTime()} style={{ gridColumn: column + 2, gridRow: day + 2 }} />;
              const entry = counts.get(dayKey(date));
              const total = (entry?.sessions ?? 0) + (entry?.projects ?? 0);
              const parts = [`${entry?.sessions ?? 0} session${entry?.sessions === 1 ? "" : "s"}`, ...(entry?.projects ? [`${entry.projects} new project${entry.projects === 1 ? "" : "s"}`] : [])];
              return (
                <span
                  key={week.getTime()}
                  className={cn("aspect-square w-full rounded-[2px] motion-reduce:[animation:none]", heatTone(total, peak))}
                  style={{ gridColumn: column + 2, gridRow: day + 2, animation: `heat-in 520ms cubic-bezier(0.16, 1, 0.3, 1) ${column * 14 + day * 10}ms both` }}
                  title={`${new Date(date).toLocaleDateString()}: ${parts.join(", ")}`}
                />
              );
            })}
          </div>
        ))}
        <div className="pointer-events-none absolute inset-y-0 right-0 left-7"><ScanSweep /></div>
      </div>
      <div className="mt-4 flex items-center justify-end gap-1.5 font-mono text-[10px] text-subtle">
        Less
        {heatTones.map((tone) => <span key={tone} className={cn("size-[11px] rounded-[2px]", tone)} />)}
        More
      </div>
    </div>
  );
}

function ProjectPipeline({ projects }: { projects: Project[] }) {
  const reduce = useReducedMotion();
  const counts = stages.map((stage) => projects.filter((project) => pipelineStage(project) === stage.id).length);
  const peak = Math.max(1, ...counts);
  return (
    <ol className="space-y-4">
      {stages.map((stage, index) => (
        <li key={stage.id} className="grid grid-cols-[minmax(0,1fr)_2.5rem] items-center gap-x-4 gap-y-1.5">
          <div className="min-w-0">
            <p className="text-sm text-foreground">{stage.label}</p>
            <p className="truncate text-xs text-subtle">{stage.hint}</p>
          </div>
          <p className="row-span-2 text-right font-mono text-lg tabular-nums text-foreground">{counts[index]}</p>
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-line-soft">
            <motion.div
              className={cn("h-full origin-left rounded-full", stage.id === "ready" ? "bg-pass" : "bg-signal")}
              initial={reduce ? false : { scaleX: 0 }}
              animate={{ scaleX: counts[index] / peak }}
              transition={{ duration: 0.9, delay: 0.2 + index * 0.08, ease: [0.16, 1, 0.3, 1] }}
            />
          </div>
        </li>
      ))}
    </ol>
  );
}

function SessionOutcomes({ sessions }: { sessions: HistorySummary[] }) {
  const reduce = useReducedMotion();
  const counts = outcomes.map((outcome) => sessions.filter((item) => item.status === outcome.status).length);
  const open = sessions.filter((item) => OPEN.has(item.status)).length;
  const finished = counts.reduce((sum, count) => sum + count, 0);
  return (
    <>
      <div className="flex h-1.5 w-full gap-0.5 overflow-hidden rounded-full bg-line-soft" role="img"
        aria-label={finished ? outcomes.map((outcome, index) => `${counts[index]} ${outcome.label.toLowerCase()}`).join(", ") : "No finished sessions yet"}>
        {finished > 0 && outcomes.map((outcome, index) => counts[index] > 0 && (
          <motion.span key={outcome.status} className={cn("h-full origin-left", outcome.tone)} style={{ width: `${(counts[index] / finished) * 100}%` }}
            initial={reduce ? false : { scaleX: 0 }} animate={{ scaleX: 1 }} transition={{ duration: 0.8, delay: 0.2 + index * 0.1, ease: [0.16, 1, 0.3, 1] }} />
        ))}
      </div>
      <ul className="mt-6 divide-y divide-line-soft">
        {outcomes.map((outcome, index) => (
          <li key={outcome.label} className="flex items-center gap-3 py-3">
            <span className={cn("size-2 rounded-full opacity-70", outcome.tone)} />
            <span className="flex-1 text-sm text-muted-foreground">{outcome.label}</span>
            <span className="w-12 text-right font-mono text-xs tabular-nums text-subtle">{finished ? `${Math.round((counts[index] / finished) * 100)}%` : "—"}</span>
            <span className="w-8 text-right font-mono text-sm tabular-nums text-foreground">{counts[index]}</span>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-xs text-subtle">{open ? `${open} session${open === 1 ? "" : "s"} still in progress, not counted above.` : "Only finished sessions are counted."}</p>
    </>
  );
}

function RecentProjects({ projects }: { projects: Project[] }) {
  const recent = [...projects].sort((a, b) => b.updated_at_ms - a.updated_at_ms).slice(0, 5);
  const stageLabel: Record<PipelineStage, string> = { "needs-code": "Code analysis pending", "needs-review": "Profile to review", "needs-probes": "Probes to connect", ready: "Ready" };
  return (
    <ul className="divide-y divide-line-soft border-y border-line-soft">
      {recent.map((project) => (
        <li key={project.id}>
          <Link href={`/projects/${encodeURIComponent(project.id)}`} className="group flex items-center gap-4 py-3.5">
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-semibold text-signal group-hover:underline">{project.name}</span>
              <span className="mt-0.5 flex items-center gap-3 text-xs text-muted-foreground">
                {project.repository ? <span className="inline-flex min-w-0 items-center gap-1 truncate"><GitBranch className="size-3 shrink-0" strokeWidth={1.5} />{project.repository.full_name}</span> : <span>{project.controller}</span>}
                <span className="shrink-0">{stageLabel[pipelineStage(project)]}</span>
              </span>
            </span>
            <span className="shrink-0 font-mono text-[11px] text-subtle">{ago(project.updated_at_ms)}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function useYearActivity(projects: Project[], sessions: HistorySummary[]) {
  return useMemo(() => {
    const since = Date.now() - WEEKS * 7 * DAY_MS;
    const counts = new Map<number, { sessions: number; projects: number }>();
    let sessionTotal = 0;
    let projectTotal = 0;
    for (const session of sessions) {
      if (session.started_at_ms < since) continue;
      const key = dayKey(session.started_at_ms);
      const entry = counts.get(key) ?? { sessions: 0, projects: 0 };
      entry.sessions += 1;
      counts.set(key, entry);
      sessionTotal += 1;
    }
    for (const project of projects) {
      if (project.created_at_ms < since) continue;
      const key = dayKey(project.created_at_ms);
      const entry = counts.get(key) ?? { sessions: 0, projects: 0 };
      entry.projects += 1;
      counts.set(key, entry);
      projectTotal += 1;
    }
    return { counts, sessionTotal, projectTotal };
  }, [projects, sessions]);
}

function DashboardSkeleton() {
  return (
    <div aria-busy="true" aria-label="Loading your activity" className="space-y-12">
      <Skeleton className="mx-auto h-40 w-full bg-surface-2 lg:w-[70%]" />
      <div className="grid grid-cols-1 gap-12 xl:grid-cols-2"><Skeleton className="h-52 bg-surface-2" /><Skeleton className="h-52 bg-surface-2" /></div>
    </div>
  );
}

export function AccountDashboardView() {
  const activity = useAccountActivity();
  const projects = activity.status === "ready" ? activity.data.projects : [];
  const sessions = activity.status === "ready" ? activity.data.sessions : [];
  const year = useYearActivity(projects, sessions);

  if (activity.status === "error") {
    return (
      <div data-tw className="mx-auto flex max-w-md flex-col items-start gap-4 py-20">
        <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border"><WifiOff className="size-5" strokeWidth={1.5} /></span>
        <div>
          <h1 className="text-lg font-semibold text-foreground">Couldn&apos;t load your activity</h1>
          <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{activity.message}</p>
        </div>
        <Button size="sm" onClick={activity.reload}><RefreshCw /> Try again</Button>
      </div>
    );
  }

  const headline = `${year.sessionTotal} diagnostic session${year.sessionTotal === 1 ? "" : "s"} in the last year`;
  return (
    <motion.div data-tw variants={stagger} initial="hidden" animate="show" className="mx-auto w-full max-w-[1100px]">
      {activity.status === "loading" ? <DashboardSkeleton /> : (
        <>
          <motion.section variants={rise} className="relative">
            <div
              className="pointer-events-none absolute -inset-x-4 -inset-y-4 opacity-60 [background-image:radial-gradient(var(--line-soft)_1px,transparent_1px)] [background-size:14px_14px] [mask-image:radial-gradient(ellipse_at_30%_40%,black,transparent_75%)]"
              aria-hidden="true"
            />
            <div className="relative">
              <SectionHead title={headline} meta={year.projectTotal ? `+ ${year.projectTotal} new project${year.projectTotal === 1 ? "" : "s"} · all projects` : "All projects"} />
              <div className="mx-auto w-full overflow-x-auto pb-1 lg:w-[70%]">
                <ActivityHeatmap counts={year.counts} />
              </div>
            </div>
          </motion.section>

          <div className="mt-12 grid grid-cols-1 gap-12 border-t border-line-soft pt-10 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)] xl:gap-0">
            <motion.section variants={rise} className="xl:pr-12">
              <SectionHead title="Project pipeline" meta={`${projects.length} project${projects.length === 1 ? "" : "s"}`} />
              {projects.length ? <ProjectPipeline projects={projects} /> : <p className="text-sm text-muted-foreground">Create a project from a GitHub repository to see where each one is in setup.</p>}
            </motion.section>

            <motion.section variants={rise} className="border-t border-line-soft pt-12 xl:border-t-0 xl:border-l xl:pt-0 xl:pl-12">
              <SectionHead title="Session outcomes" meta={`${sessions.length} total · all time`} />
              <SessionOutcomes sessions={sessions} />
            </motion.section>
          </div>

          <motion.section variants={rise} className="mt-12 border-t border-line-soft pt-10">
            <SectionHead title="Recent projects" meta={projects.length > 5 ? `5 of ${projects.length}` : undefined} />
            {projects.length ? (
              <>
                <RecentProjects projects={projects} />
                <Button asChild variant="ghost" size="sm" className="mt-3"><Link href="/projects">All projects <ArrowUpRight /></Link></Button>
              </>
            ) : (
              <div className="flex flex-col items-start gap-4">
                <span className="grid size-10 place-items-center rounded-lg border border-border bg-surface-2 text-muted-foreground">
                  <FolderGit2 className="size-[18px]" strokeWidth={1.5} />
                </span>
                <div>
                  <h3 className="text-sm font-semibold text-foreground">No projects on the bench yet</h3>
                  <p className="mt-1 max-w-[40ch] text-sm leading-relaxed text-muted-foreground">Projects you create from a repository line up here, most recent first.</p>
                </div>
                <Button asChild variant="outline" size="sm" className="active:scale-[0.98]"><Link href="/projects">Go to projects <ArrowUpRight /></Link></Button>
              </div>
            )}
          </motion.section>
        </>
      )}
    </motion.div>
  );
}
