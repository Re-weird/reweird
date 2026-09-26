"use client";

import { memo, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AnimatePresence, LayoutGroup, motion, useReducedMotion } from "framer-motion";
import { ArrowUpRight, FolderGit2, Plus, Search } from "lucide-react";
import type { HistorySummary, Project } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { historyApi, projectApi } from "@/lib/api";
import { cn } from "@/lib/utils";

const WEEKS = 12;
const WEEK_MS = 7 * 86_400_000;
const EASE = [0.32, 0.72, 0, 1] as const;
const spring = { type: "spring", stiffness: 110, damping: 20 } as const;
const snappy = { type: "spring", stiffness: 380, damping: 32 } as const;

type StatusKey = "confirmed" | "progress" | "failed" | "awaiting";

const statusMeta: Record<Project["analysis_status"], { key: StatusKey; label: string }> = {
  CONFIRMED: { key: "confirmed", label: "Profile confirmed" },
  DRAFT_READY: { key: "progress", label: "Draft ready" },
  PROCESSING: { key: "progress", label: "Analyzing" },
  PENDING: { key: "awaiting", label: "Awaiting analysis" },
  FAILED: { key: "failed", label: "Analysis failed" },
};

const statusFilters: { key: StatusKey | "all"; label: string }[] = [
  { key: "all", label: "All" },
  { key: "confirmed", label: "Confirmed" },
  { key: "progress", label: "In progress" },
  { key: "failed", label: "Failed" },
  { key: "awaiting", label: "Awaiting" },
];

const statusDot: Record<StatusKey, string> = {
  confirmed: "bg-pass",
  progress: "bg-warn",
  failed: "bg-fail",
  awaiting: "bg-subtle",
};

const reveal = {
  hidden: { opacity: 0, y: 18, filter: "blur(6px)" },
  show: { opacity: 1, y: 0, filter: "blur(0px)", transition: { duration: 0.8, ease: EASE } },
};

function ago(ms: number) {
  const seconds = Math.max(0, (Date.now() - ms) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  return months < 12 ? `${months}mo ago` : `${Math.floor(months / 12)}y ago`;
}

// Weekly session counts from stored history, oldest week first.
function weeklyActivity(items: HistorySummary[]) {
  const now = Date.now();
  const total = Array<number>(WEEKS).fill(0);
  const byProject = new Map<string, number[]>();
  for (const item of items) {
    const weeksAgo = Math.floor((now - item.started_at_ms) / WEEK_MS);
    if (weeksAgo < 0 || weeksAgo >= WEEKS) continue;
    const index = WEEKS - 1 - weeksAgo;
    const series = byProject.get(item.project_id) ?? Array<number>(WEEKS).fill(0);
    series[index] += 1;
    total[index] += 1;
    byProject.set(item.project_id, series);
  }
  return { total, byProject };
}

function weekLabels() {
  const now = Date.now();
  return Array.from({ length: WEEKS }, (_, index) => {
    const date = new Date(now - (WEEKS - 1 - index) * WEEK_MS);
    return date.toLocaleDateString("en", { month: "short", day: "numeric" });
  });
}

/* GitHub-Insights-style weekly bars across every project. */
const ActivityBars = memo(function ActivityBars({ series }: { series: number[] }) {
  const reduce = useReducedMotion();
  const labels = useMemo(() => weekLabels(), []);
  const max = Math.max(...series, 1);
  return (
    <div>
      <div className="flex h-28 items-end gap-1.5" role="img" aria-label={`Weekly diagnostic sessions over ${WEEKS} weeks: ${series.join(", ")}`}>
        {series.map((value, index) => {
          const height = value === 0 ? 3 : Math.max(8, (value / max) * 112);
          return (
            <div key={index} className="group relative flex h-full flex-1 items-end">
              <motion.span
                className={cn("block w-full origin-bottom rounded-[3px]", value === 0 ? "bg-graph/25" : "bg-graph/85 group-hover:bg-graph")}
                style={{ height }}
                initial={reduce ? false : { scaleY: 0 }}
                animate={{ scaleY: 1 }}
                transition={{ duration: 0.7, delay: 0.25 + index * 0.035, ease: EASE }}
              />
              <span className="pointer-events-none absolute bottom-full left-1/2 mb-2 -translate-x-1/2 rounded-md border border-border bg-popover px-2 py-1 font-mono text-[10px] whitespace-nowrap text-foreground opacity-0 shadow-[0_8px_24px_-12px_rgba(0,0,0,0.6)] transition-opacity duration-300 group-hover:opacity-100">
                {value} · wk of {labels[index]}
              </span>
            </div>
          );
        })}
      </div>
      <div className="mt-3 flex justify-between font-mono text-[10px] text-subtle">
        <span>{labels[0]}</span>
        <span>{labels[Math.floor(WEEKS / 2)]}</span>
        <span>This week</span>
      </div>
    </div>
  );
});

/* GitHub-style activity graph for one project, in the contrasting graph color. */
const Sparkline = memo(function Sparkline({ series, id }: { series: number[]; id: string }) {
  const reduce = useReducedMotion();
  const width = 200;
  const height = 44;
  const total = series.reduce((sum, value) => sum + value, 0);
  const max = Math.max(...series, 1);
  const step = width / (series.length - 1);
  const points = series.map((value, index) => [index * step, height - 5 - (value / max) * (height - 12)] as const);
  const line = points.map(([x, y], index) => `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${line} L${width},${height} L0,${height} Z`;
  const gradient = `spark-${id}`;
  const last = points[points.length - 1];

  return (
    <div className="flex flex-col items-end gap-1.5">
      <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible" role="img" aria-label={`${total} diagnostic sessions in the last ${WEEKS} weeks`}>
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--graph)" stopOpacity={total > 0 ? 0.35 : 0.14} />
            <stop offset="100%" stopColor="var(--graph)" stopOpacity="0" />
          </linearGradient>
        </defs>
        <motion.path d={area} fill={`url(#${gradient})`} initial={reduce ? false : { opacity: 0 }} animate={{ opacity: 1 }} transition={{ duration: 0.8, delay: 0.3, ease: EASE }} />
        <motion.path
          d={line}
          fill="none"
          className={total > 0 ? "stroke-graph" : "stroke-graph/60"}
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          initial={reduce ? false : { pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: 1, ease: EASE }}
        />
        {points.map(([x, y], index) => (
          <circle key={index} cx={x} cy={y} r={index === points.length - 1 ? 3.25 : 1.6} className={index === points.length - 1 ? "fill-graph stroke-chrome" : "fill-graph/55"} strokeWidth={index === points.length - 1 ? 1.5 : 0} />
        ))}
        {total > 0 && !reduce && (
          <motion.circle cx={last[0]} cy={last[1]} r="3.25" className="fill-graph" animate={{ r: [3.25, 9], opacity: [0.45, 0] }} transition={{ duration: 2.2, repeat: Infinity, ease: "easeOut" }} />
        )}
      </svg>
      <span className="font-mono text-[10px] text-subtle"><span className="text-foreground">{total}</span> session{total === 1 ? "" : "s"} · {WEEKS} wk</span>
    </div>
  );
});

function ProjectRow({ project, series }: { project: Project; series: number[] }) {
  const meta = statusMeta[project.analysis_status];
  return (
    <motion.li
      layout
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -6, transition: { duration: 0.18 } }}
      transition={spring}
      className="border-b border-line-soft last:border-b-0"
    >
      <Link
        href={`/projects/${project.id}`}
        className="group -mx-4 grid grid-cols-1 items-center gap-5 rounded-lg px-4 py-6 transition-colors duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] hover:bg-surface/70 sm:grid-cols-[minmax(0,1fr)_auto]"
      >
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h3 className="truncate text-base font-semibold tracking-tight text-foreground transition-colors duration-300 group-hover:text-signal">{project.name}</h3>
            <span className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground ring-1 ring-border">
              <span className={cn("size-1.5 rounded-full", statusDot[meta.key])} />
              {meta.label}
            </span>
          </div>
          <p className="mt-2 line-clamp-1 max-w-[68ch] text-sm leading-relaxed text-muted-foreground">{project.description || "No description provided."}</p>
          <div className="mt-3.5 flex flex-wrap items-center gap-x-6 gap-y-1.5 text-xs text-subtle">
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-signal/70" />{project.controller}</span>
            <span className="font-mono">{project.logic_voltage} V logic</span>
            <span className="font-mono">Updated {ago(project.updated_at_ms)}</span>
          </div>
        </div>
        <div className="flex items-center gap-5 justify-self-start sm:justify-self-end">
          <Sparkline series={series} id={project.id} />
          <span className="hidden size-8 place-items-center rounded-md text-subtle ring-1 ring-line-soft transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:-translate-y-px group-hover:translate-x-0.5 group-hover:text-foreground group-hover:ring-border sm:grid">
            <ArrowUpRight className="size-4" strokeWidth={1.5} />
          </span>
        </div>
      </Link>
    </motion.li>
  );
}

function LoadingRows() {
  return (
    <ul aria-busy="true" aria-label="Loading projects">
      {[0, 1, 2].map((row) => (
        <li key={row} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-5 border-b border-line-soft py-6 last:border-b-0">
          <div className="space-y-3">
            <Skeleton className="h-4 w-56 bg-surface-2" />
            <Skeleton className="h-3 w-80 max-w-full bg-surface-2" />
            <Skeleton className="h-3 w-44 bg-surface-2" />
          </div>
          <Skeleton className="h-9 w-40 bg-surface-2" />
        </li>
      ))}
    </ul>
  );
}

export function ProjectDashboardView({ onNewProject }: { onNewProject: () => void }) {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [history, setHistory] = useState<HistorySummary[]>([]);
  const [error, setError] = useState("");
  const [status, setStatus] = useState<StatusKey | "all">("all");
  const [controller, setController] = useState("all");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<"updated" | "name">("updated");

  useEffect(() => {
    let live = true;
    projectApi.listProjects()
      .then((items) => { if (live) setProjects(items ?? []); })
      .catch((cause) => { if (live) { setError(cause instanceof Error ? cause.message : "Projects are unavailable."); setProjects([]); } });
    historyApi.list()
      .then((response) => { if (live) setHistory(response.items ?? []); })
      .catch(() => undefined);
    return () => { live = false; };
  }, []);

  const activity = useMemo(() => weeklyActivity(history), [history]);
  const emptySeries = useMemo(() => Array<number>(WEEKS).fill(0), []);
  const totalSessions = activity.total.reduce((sum, value) => sum + value, 0);
  const activeProjects = activity.byProject.size;

  const counts = useMemo(() => {
    const result: Record<StatusKey | "all", number> = { all: 0, confirmed: 0, progress: 0, failed: 0, awaiting: 0 };
    for (const project of projects ?? []) { result.all++; result[statusMeta[project.analysis_status].key]++; }
    return result;
  }, [projects]);

  const controllers = useMemo(() => Array.from(new Set((projects ?? []).map((project) => project.controller))).sort(), [projects]);

  const filtered = useMemo(() => {
    let list = projects ?? [];
    if (status !== "all") list = list.filter((project) => statusMeta[project.analysis_status].key === status);
    if (controller !== "all") list = list.filter((project) => project.controller === controller);
    const term = search.trim().toLowerCase();
    if (term) list = list.filter((project) => project.name.toLowerCase().includes(term) || project.description?.toLowerCase().includes(term));
    return [...list].sort((a, b) => sort === "name" ? a.name.localeCompare(b.name) : b.updated_at_ms - a.updated_at_ms);
  }, [projects, status, controller, search, sort]);

  const hasProjects = (projects?.length ?? 0) > 0;
  const selectClass = "h-9 rounded-md border border-input bg-transparent px-3 text-[13px] text-foreground outline-none transition-colors hover:border-border focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/40";

  return (
    <motion.div
      data-tw
      className="mx-auto w-full max-w-[1120px] pb-16"
      initial="hidden"
      animate="show"
      variants={{ hidden: {}, show: { transition: { staggerChildren: 0.09 } } }}
    >
      {/* Heading */}
      <motion.div variants={reveal} className="flex flex-col gap-6 pt-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="flex items-baseline gap-3 text-3xl font-semibold tracking-tight text-foreground">
            Projects
            <span className="font-mono text-base font-normal tabular-nums text-subtle">{projects ? counts.all : "—"}</span>
          </h1>
          <p className="mt-2 max-w-[54ch] text-sm leading-relaxed text-muted-foreground">Every project analyzed on this instance, each with its last twelve weeks of diagnostic sessions.</p>
        </div>
        <Button onClick={onNewProject} className="group h-10 self-start rounded-md pr-1.5 pl-4 transition-transform active:scale-[0.98] md:self-auto">
          New project
          <span className="ml-1 grid size-7 place-items-center rounded-[5px] bg-white/15 transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:scale-105 group-hover:rotate-90">
            <Plus className="size-4" strokeWidth={2} />
          </span>
        </Button>
      </motion.div>

      {/* Activity overview: double-bezel panel */}
      <motion.section variants={reveal} className="mt-10 rounded-xl bg-surface-2/50 p-1.5 ring-1 ring-line-soft" aria-labelledby="activity-heading">
        <div className="grid grid-cols-1 gap-8 rounded-lg bg-surface p-6 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] md:grid-cols-[220px_minmax(0,1fr)] md:p-8">
          <div className="flex flex-col justify-between gap-6">
            <div>
              <h2 id="activity-heading" className="font-mono text-[10px] tracking-[0.18em] text-subtle uppercase">Activity · {WEEKS} weeks</h2>
              <p className="mt-3 font-mono text-5xl leading-none font-light tracking-tighter tabular-nums text-foreground">{totalSessions}</p>
              <p className="mt-2 text-xs text-muted-foreground">diagnostic sessions</p>
            </div>
            <dl className="grid grid-cols-2 gap-4 border-t border-line-soft pt-4">
              <div>
                <dt className="text-[11px] text-subtle">Active projects</dt>
                <dd className="mt-1 font-mono text-lg tabular-nums text-foreground">{activeProjects}</dd>
              </div>
              <div>
                <dt className="text-[11px] text-subtle">Confirmed</dt>
                <dd className="mt-1 font-mono text-lg tabular-nums text-foreground">{projects ? counts.confirmed : "—"}</dd>
              </div>
            </dl>
          </div>
          <ActivityBars series={activity.total} />
        </div>
      </motion.section>

      {/* Filters */}
      <motion.div variants={reveal} className="mt-12 flex flex-col gap-3 lg:flex-row lg:items-center">
        <label className="relative flex-1">
          <span className="sr-only">Find a project</span>
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-subtle" strokeWidth={1.5} />
          <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Find a project…" className="h-9 pl-9 text-[13px]" />
        </label>
        <div className="flex flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="controller-filter">Controller</label>
          <select id="controller-filter" value={controller} onChange={(event) => setController(event.target.value)} className={selectClass}>
            <option value="all">All controllers</option>
            {controllers.map((name) => <option key={name} value={name}>{name}</option>)}
          </select>
          <label className="sr-only" htmlFor="sort-order">Sort</label>
          <select id="sort-order" value={sort} onChange={(event) => setSort(event.target.value as "updated" | "name")} className={selectClass}>
            <option value="updated">Last updated</option>
            <option value="name">Name</option>
          </select>
        </div>
      </motion.div>

      <motion.div variants={reveal}>
        <LayoutGroup id="status-filter">
          <div role="tablist" aria-label="Filter by status" className="mt-4 flex flex-wrap gap-1 border-b border-border pb-3">
            {statusFilters.map((filter) => {
              const active = status === filter.key;
              return (
                <button
                  key={filter.key}
                  role="tab"
                  aria-selected={active}
                  onClick={() => setStatus(filter.key)}
                  className={cn("relative rounded-md px-3 py-1.5 text-[13px] transition-colors duration-300", active ? "font-semibold text-foreground" : "text-muted-foreground hover:text-foreground")}
                >
                  {active && <motion.span layoutId="status-pill" transition={snappy} className="absolute inset-0 rounded-md bg-accent ring-1 ring-line-soft" />}
                  <span className="relative flex items-center gap-2">
                    {filter.label}
                    <span className="font-mono text-[11px] tabular-nums text-subtle">{projects ? counts[filter.key] : "—"}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </LayoutGroup>

        {error && <p role="alert" className="mt-6 rounded-md px-3 py-2 text-sm text-fail ring-1 ring-fail/40">{error}</p>}

        <div className="mt-2">
          {projects === null ? <LoadingRows /> : !hasProjects ? (
            <div className="grid grid-cols-1 items-center gap-10 py-16 md:grid-cols-2">
              <div className="flex flex-col gap-2" aria-hidden="true">
                {[1, 0.55, 0.25].map((opacity) => (
                  <div key={opacity} className="flex items-center gap-3 border-b border-line-soft py-4" style={{ opacity }}>
                    <span className="h-2.5 w-44 rounded-full bg-line-soft" />
                    <span className="ml-auto h-6 w-28 rounded bg-line-soft" />
                  </div>
                ))}
              </div>
              <div className="flex flex-col items-start gap-4">
                <span className="grid size-11 place-items-center rounded-lg bg-surface-2 text-muted-foreground ring-1 ring-border"><FolderGit2 className="size-5" strokeWidth={1.5} /></span>
                <div>
                  <h2 className="text-base font-semibold text-foreground">Nothing on the bench yet</h2>
                  <p className="mt-1 max-w-[42ch] text-sm leading-relaxed text-muted-foreground">Upload a project&apos;s code and an optional hardware photo. ReWeird drafts a profile for you to confirm.</p>
                </div>
                <Button onClick={onNewProject} className="active:scale-[0.98]"><Plus /> New project</Button>
              </div>
            </div>
          ) : filtered.length === 0 ? (
            <div className="py-16 text-center">
              <p className="text-sm font-semibold text-foreground">No matching projects</p>
              <p className="mt-1 text-sm text-muted-foreground">Try a different filter or search term.</p>
            </div>
          ) : (
            <ul>
              <AnimatePresence initial={false} mode="popLayout">
                {filtered.map((project) => <ProjectRow key={project.id} project={project} series={activity.byProject.get(project.id) ?? emptySeries} />)}
              </AnimatePresence>
            </ul>
          )}
        </div>
      </motion.div>
    </motion.div>
  );
}
