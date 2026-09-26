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
const spring = { type: "spring", stiffness: 100, damping: 20 } as const;

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

// Weekly session counts per project, oldest first, from stored history.
function weeklyActivity(items: HistorySummary[]) {
  const now = Date.now();
  const byProject = new Map<string, number[]>();
  for (const item of items) {
    const weeksAgo = Math.floor((now - item.started_at_ms) / WEEK_MS);
    if (weeksAgo < 0 || weeksAgo >= WEEKS) continue;
    const series = byProject.get(item.project_id) ?? Array<number>(WEEKS).fill(0);
    series[WEEKS - 1 - weeksAgo] += 1;
    byProject.set(item.project_id, series);
  }
  return byProject;
}

const ActivityGraph = memo(function ActivityGraph({ series }: { series: number[] }) {
  const reduce = useReducedMotion();
  const width = 156;
  const height = 34;
  const total = series.reduce((sum, value) => sum + value, 0);
  const max = Math.max(...series, 1);
  const step = width / (series.length - 1);
  const points = series.map((value, index) => [index * step, height - 3 - (value / max) * (height - 8)] as const);
  const line = points.map(([x, y], index) => `${index ? "L" : "M"}${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
  const area = `${line} L${width},${height} L0,${height} Z`;

  return (
    <div className="flex flex-col items-end gap-1">
      <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible" role="img" aria-label={`${total} diagnostic sessions in the last ${WEEKS} weeks`}>
        <line x1="0" x2={width} y1={height - 0.5} y2={height - 0.5} className="stroke-line-soft" strokeWidth="1" />
        {total > 0 && <path d={area} className="fill-signal/10" />}
        <motion.path
          d={line}
          fill="none"
          className={total > 0 ? "stroke-signal" : "stroke-line-soft"}
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          initial={reduce ? false : { pathLength: 0 }}
          animate={{ pathLength: 1 }}
          transition={{ duration: 0.9, ease: [0.16, 1, 0.3, 1] }}
        />
        {total > 0 && <circle cx={points[points.length - 1][0]} cy={points[points.length - 1][1]} r="2.5" className="fill-signal stroke-chrome" strokeWidth="1.5" />}
      </svg>
      <span className="font-mono text-[10px] text-subtle">{total} sessions · {WEEKS} wk</span>
    </div>
  );
});

function ProjectRow({ project, series }: { project: Project; series: number[] }) {
  const meta = statusMeta[project.analysis_status];
  return (
    <motion.li
      layout
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -6, transition: { duration: 0.15 } }}
      transition={spring}
      className="border-b border-line-soft last:border-b-0"
    >
      <Link
        href={`/projects/${project.id}`}
        className="group -mx-3 grid grid-cols-1 items-center gap-4 rounded-lg px-3 py-5 transition-colors hover:bg-accent/40 sm:grid-cols-[minmax(0,1fr)_auto]"
      >
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2.5">
            <h3 className="truncate text-[15px] font-semibold tracking-tight text-foreground transition-colors group-hover:text-signal">{project.name}</h3>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-border px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
              <span className={cn("size-1.5 rounded-full", statusDot[meta.key])} />
              {meta.label}
            </span>
          </div>
          <p className="mt-1.5 line-clamp-1 max-w-[70ch] text-sm text-muted-foreground">{project.description || "No description provided."}</p>
          <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-subtle">
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-signal/70" />{project.controller}</span>
            <span className="font-mono">{project.logic_voltage} V logic</span>
            <span className="font-mono">Updated {ago(project.updated_at_ms)}</span>
          </div>
        </div>
        <div className="flex items-center gap-4 justify-self-start sm:justify-self-end">
          <ActivityGraph series={series} />
          <ArrowUpRight className="hidden size-4 -translate-x-1 text-subtle opacity-0 transition-all group-hover:translate-x-0 group-hover:opacity-100 sm:block" />
        </div>
      </Link>
    </motion.li>
  );
}

function LoadingRows() {
  return (
    <ul aria-busy="true" aria-label="Loading projects">
      {[0, 1, 2].map((row) => (
        <li key={row} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 border-b border-line-soft py-5 last:border-b-0">
          <div className="space-y-2.5">
            <Skeleton className="h-4 w-56 bg-surface-2" />
            <Skeleton className="h-3 w-80 max-w-full bg-surface-2" />
            <Skeleton className="h-3 w-40 bg-surface-2" />
          </div>
          <Skeleton className="h-8 w-36 bg-surface-2" />
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
  const selectClass = "h-9 rounded-md border border-input bg-transparent px-2.5 text-[13px] text-foreground outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50";

  return (
    <motion.div
      className="mx-auto w-full max-w-[1100px]"
      initial="hidden"
      animate="show"
      variants={{ hidden: {}, show: { transition: { staggerChildren: 0.08 } } }}
    >
      <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0, transition: spring } }} className="flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-subtle">Workspace</p>
          <h1 className="mt-1.5 flex items-baseline gap-3 text-2xl font-semibold tracking-tight text-foreground">
            Projects
            <span className="font-mono text-sm font-normal text-subtle tabular-nums">{projects ? counts.all : "—"}</span>
          </h1>
          <p className="mt-1.5 max-w-[56ch] text-sm leading-relaxed text-muted-foreground">Every project analyzed on this instance, with its last 12 weeks of diagnostic sessions.</p>
        </div>
        <Button onClick={onNewProject} className="self-start active:scale-[0.98] md:self-auto"><Plus /> New project</Button>
      </motion.div>

      <motion.div variants={{ hidden: { opacity: 0, y: 12 }, show: { opacity: 1, y: 0, transition: spring } }} className="mt-8 flex flex-col gap-3 border-b border-border pb-4 lg:flex-row lg:items-center">
        <label className="relative flex-1">
          <span className="sr-only">Find a project</span>
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-subtle" />
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

      <motion.div variants={{ hidden: { opacity: 0 }, show: { opacity: 1, transition: spring } }}>
        <LayoutGroup id="status-filter">
          <div role="tablist" aria-label="Filter by status" className="mt-4 flex flex-wrap gap-1">
            {statusFilters.map((filter) => {
              const active = status === filter.key;
              return (
                <button
                  key={filter.key}
                  role="tab"
                  aria-selected={active}
                  onClick={() => setStatus(filter.key)}
                  className={cn("relative rounded-md px-3 py-1.5 text-[13px] transition-colors", active ? "font-semibold text-foreground" : "text-muted-foreground hover:text-foreground")}
                >
                  {active && <motion.span layoutId="status-pill" transition={{ type: "spring", stiffness: 380, damping: 32 }} className="absolute inset-0 rounded-md bg-accent" />}
                  <span className="relative flex items-center gap-2">
                    {filter.label}
                    <span className="font-mono text-[11px] text-subtle tabular-nums">{projects ? counts[filter.key] : "—"}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </LayoutGroup>

        {error && <p role="alert" className="mt-6 rounded-md border border-fail/40 px-3 py-2 text-sm text-fail">{error}</p>}

        <div className="mt-2">
          {projects === null ? <LoadingRows /> : !hasProjects ? (
            <div className="grid grid-cols-1 items-center gap-10 py-16 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
              <div className="flex flex-col gap-2" aria-hidden="true">
                {[1, 0.55, 0.25].map((opacity) => (
                  <div key={opacity} className="flex items-center gap-3 border-b border-line-soft py-4" style={{ opacity }}>
                    <span className="h-2.5 w-44 rounded-full bg-line-soft" />
                    <span className="ml-auto h-6 w-28 rounded bg-line-soft" />
                  </div>
                ))}
              </div>
              <div className="flex flex-col items-start gap-4">
                <span className="grid size-11 place-items-center rounded-lg border border-border bg-surface-2 text-muted-foreground"><FolderGit2 className="size-5" strokeWidth={1.5} /></span>
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
                {filtered.map((project) => <ProjectRow key={project.id} project={project} series={activity.get(project.id) ?? emptySeries} />)}
              </AnimatePresence>
            </ul>
          )}
        </div>
      </motion.div>
    </motion.div>
  );
}
