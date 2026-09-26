"use client";

import { memo, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { ArrowUpRight, FolderGit2, Globe, Lock, Plus, Search } from "lucide-react";
import type { HistorySummary, Project } from "@reweird/shared-types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { historyApi, projectApi } from "@/lib/api";
import { cn } from "@/lib/utils";

const WEEKS = 12;
const WEEK_MS = 7 * 86_400_000;
const EASE = [0.32, 0.72, 0, 1] as const;
const spring = { type: "spring", stiffness: 110, damping: 20 } as const;

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


// Catmull-Rom through the points, as cubic Beziers: smooth, but still peaks
// exactly at each week's value (no overshoot below the baseline).
function smoothPath(points: readonly (readonly [number, number])[], floor: number) {
  return points.map(([x, y], index) => {
    if (index === 0) return `M${x.toFixed(1)},${y.toFixed(1)}`;
    const [x0, y0] = points[index - 2] ?? points[index - 1];
    const [x1, y1] = points[index - 1];
    const [x3, y3] = points[index + 1] ?? [x, y];
    const c1x = x1 + (x - x0) / 6, c1y = Math.min(floor, y1 + (y - y0) / 6);
    const c2x = x - (x3 - x1) / 6, c2y = Math.min(floor, y - (y3 - y1) / 6);
    return `C${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${x.toFixed(1)},${y.toFixed(1)}`;
  }).join(" ");
}

/* GitHub repo-list activity graph: a single smoothed line whose stroke
   brightens with height (vertical gradient), no fill, dots, or labels. */
const Sparkline = memo(function Sparkline({ series, id }: { series: number[]; id: string }) {
  const reduce = useReducedMotion();
  const width = 156;
  const height = 30;
  const floor = height - 2;
  const total = series.reduce((sum, value) => sum + value, 0);
  const max = Math.max(...series, 1);
  const step = width / (series.length - 1);
  const points = series.map((value, index) => [index * step, floor - (value / max) * (height - 6)] as const);
  const gradient = `spark-${id}`;

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible" role="img" aria-label={`${total} diagnostic session${total === 1 ? "" : "s"} in the last ${WEEKS} weeks`}>
      <defs>
        <linearGradient id={gradient} gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="0" y2={height}>
          <stop offset="0%" stopColor="var(--graph)" />
          <stop offset="100%" stopColor="var(--graph)" stopOpacity="0.45" />
        </linearGradient>
      </defs>
      <motion.path
        d={smoothPath(points, floor)}
        fill="none"
        stroke={`url(#${gradient})`}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        initial={reduce ? false : { pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 1.1, ease: EASE }}
      />
    </svg>
  );
});

function VisibilityToggle({ project, onChange }: { project: Project; onChange: (next: Project["visibility"]) => void }) {
  const isPublic = project.visibility === "public";
  const next = isPublic ? "private" : "public";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={() => onChange(next)}
          aria-label={`${isPublic ? "Public" : "Private"} project. Make ${next}.`}
          className={cn(
            "relative z-10 inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-medium ring-1 transition-colors duration-300 active:scale-95",
            isPublic ? "text-graph ring-graph/50 hover:bg-graph/10" : "text-muted-foreground ring-border hover:bg-accent hover:text-foreground",
          )}
        >
          {isPublic ? <Globe className="size-3" strokeWidth={1.75} /> : <Lock className="size-3" strokeWidth={1.75} />}
          {isPublic ? "Public" : "Private"}
        </button>
      </TooltipTrigger>
      <TooltipContent>Make {next}</TooltipContent>
    </Tooltip>
  );
}

function ProjectRow({ project, series, onVisibility }: { project: Project; series: number[]; onVisibility: (id: string, next: Project["visibility"]) => void }) {
  return (
    <motion.li
      layout
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -6, transition: { duration: 0.18 } }}
      transition={spring}
      className="border-b border-line-soft last:border-b-0"
    >
      <div className="group relative -mx-4 grid grid-cols-1 items-center gap-5 rounded-lg px-4 py-6 transition-colors duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] hover:bg-surface/70 sm:grid-cols-[minmax(0,1fr)_auto]">
        {/* Row-wide link sits under the content; the visibility toggle is a separate button above it (no button nested inside a link). */}
        <Link href={`/projects/${project.id}`} aria-label={`Open ${project.name}`} className="absolute inset-0 rounded-lg focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none" />
        <div className="pointer-events-none min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h3 className="truncate text-[17px] font-bold tracking-tight text-signal group-hover:underline group-hover:underline-offset-4">{project.name}</h3>
            <span className="pointer-events-auto"><VisibilityToggle project={project} onChange={(next) => onVisibility(project.id, next)} /></span>
          </div>
          <p className="mt-2 line-clamp-1 max-w-[68ch] text-sm leading-relaxed text-muted-foreground">{project.description || "No description provided."}</p>
          <div className="mt-3.5 flex flex-wrap items-center gap-x-6 gap-y-1.5 text-xs text-subtle">
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-signal/70" />{project.controller}</span>
            <span className="font-mono">{project.logic_voltage} V logic</span>
            <span className="font-mono">Updated {ago(project.updated_at_ms)}</span>
          </div>
        </div>
        <div className="pointer-events-none flex items-center gap-5 justify-self-start sm:justify-self-end">
          <Sparkline series={series} id={project.id} />
          <span className="hidden size-8 place-items-center rounded-md text-subtle ring-1 ring-line-soft transition-all duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:-translate-y-px group-hover:translate-x-0.5 group-hover:text-foreground group-hover:ring-border sm:grid">
            <ArrowUpRight className="size-4" strokeWidth={1.5} />
          </span>
        </div>
      </div>
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

  const changeVisibility = async (id: string, next: Project["visibility"]) => {
    const previous = projects?.find((project) => project.id === id)?.visibility;
    setProjects((list) => list?.map((project) => project.id === id ? { ...project, visibility: next } : project) ?? list);
    try {
      await projectApi.setVisibility(id, next);
      setError("");
    } catch (cause) {
      setProjects((list) => list?.map((project) => project.id === id && previous ? { ...project, visibility: previous } : project) ?? list);
      setError(cause instanceof Error ? cause.message : "Visibility could not be changed.");
    }
  };

  const activity = useMemo(() => weeklyActivity(history), [history]);
  const emptySeries = useMemo(() => Array<number>(WEEKS).fill(0), []);

  const controllers = useMemo(() => Array.from(new Set((projects ?? []).map((project) => project.controller))).sort(), [projects]);

  const filtered = useMemo(() => {
    let list = projects ?? [];
    if (controller !== "all") list = list.filter((project) => project.controller === controller);
    const term = search.trim().toLowerCase();
    if (term) list = list.filter((project) => project.name.toLowerCase().includes(term) || project.description?.toLowerCase().includes(term));
    return [...list].sort((a, b) => sort === "name" ? a.name.localeCompare(b.name) : b.updated_at_ms - a.updated_at_ms);
  }, [projects, controller, search, sort]);

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
      {/* Filters */}
      <motion.div variants={reveal} className="flex flex-col gap-3 border-b border-border pb-4 lg:flex-row lg:items-center">
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
          <Button onClick={onNewProject} className="group h-9 rounded-md pr-1 pl-3.5 transition-transform active:scale-[0.98]">
            New
            <span className="ml-0.5 grid size-7 place-items-center rounded-[5px] bg-white/15 transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] group-hover:rotate-90">
              <Plus className="size-4" strokeWidth={2} />
            </span>
          </Button>
        </div>
      </motion.div>

      <motion.div variants={reveal}>
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
                {filtered.map((project) => <ProjectRow key={project.id} project={project} series={activity.byProject.get(project.id) ?? emptySeries} onVisibility={changeVisibility} />)}
              </AnimatePresence>
            </ul>
          )}
        </div>
      </motion.div>
    </motion.div>
  );
}
