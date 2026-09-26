"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronRight, FolderGit2, Search, Upload } from "lucide-react";
import type { Project } from "@reweird/shared-types";
import { projectApi } from "@/lib/api";

const date = (value?: number) => value ? new Date(value).toLocaleString() : "—";

type StatusKey = "stable" | "active" | "intermittent" | "idle";

const statusMeta: Record<Project["analysis_status"], { dot: StatusKey; label: string }> = {
  CONFIRMED: { dot: "stable", label: "Profile confirmed" },
  DRAFT_READY: { dot: "active", label: "Draft ready for review" },
  PROCESSING: { dot: "active", label: "Analyzing" },
  PENDING: { dot: "idle", label: "Awaiting analysis" },
  FAILED: { dot: "intermittent", label: "Analysis failed" },
};

const statusOrder: StatusKey[] = ["stable", "active", "intermittent", "idle"];
const statusFilterLabel: Record<StatusKey, string> = { stable: "Confirmed", active: "In progress", intermittent: "Failed", idle: "Awaiting analysis" };

export function ProjectDashboardView({ onOpenProject, onNewProject }: { onOpenProject: (project: Project) => void; onNewProject: () => void }) {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusKey | "all">("all");
  const [controllerFilter, setControllerFilter] = useState<string>("all");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<"updated" | "name">("updated");

  useEffect(() => {
    let live = true;
    projectApi.listProjects().then((items) => { if (live) setProjects(items ?? []); })
      .catch((cause) => { if (live) { setError(cause instanceof Error ? cause.message : "Projects are unavailable."); setProjects([]); } });
    return () => { live = false; };
  }, []);

  const statusCounts = useMemo(() => {
    const counts: Record<StatusKey, number> = { stable: 0, active: 0, intermittent: 0, idle: 0 };
    for (const project of projects ?? []) counts[statusMeta[project.analysis_status].dot]++;
    return counts;
  }, [projects]);

  const controllerCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const project of projects ?? []) counts.set(project.controller, (counts.get(project.controller) ?? 0) + 1);
    return Array.from(counts.entries()).sort((a, b) => b[1] - a[1]);
  }, [projects]);

  const filtered = useMemo(() => {
    let list = projects ?? [];
    if (statusFilter !== "all") list = list.filter((project) => statusMeta[project.analysis_status].dot === statusFilter);
    if (controllerFilter !== "all") list = list.filter((project) => project.controller === controllerFilter);
    if (search.trim()) { const term = search.trim().toLowerCase(); list = list.filter((project) => project.name.toLowerCase().includes(term)); }
    return [...list].sort((a, b) => sort === "name" ? a.name.localeCompare(b.name) : b.updated_at_ms - a.updated_at_ms);
  }, [projects, statusFilter, controllerFilter, search, sort]);

  return <div className="project-dashboard">
    <section className="page-heading">
      <div><span className="kicker">Workspace</span><h1>Your projects</h1><p>Every project analyzed on this ReWeird instance. Open one to review its evidence, or upload a new project to start a profile.</p></div>
      <button className="primary" onClick={onNewProject}><Upload size={15} /> Upload a project</button>
    </section>

    {error && <div className="form-error page-error">{error}</div>}

    {projects === null ? <div className="panel"><div className="empty-state"><FolderGit2 size={24} /><h2>Loading projects…</h2></div></div>
      : projects.length === 0 ? <div className="panel"><div className="empty-state"><FolderGit2 size={24} /><h2>No projects yet</h2><p>Upload a project's code and an optional hardware photo to build its first profile.</p><button className="primary" onClick={onNewProject}><Upload size={15} /> Upload a project</button></div></div>
      : <div className="project-dash-layout">
          <aside className="project-dash-filters">
            <div>
              <p className="nav-label">Status</p>
              <button className={statusFilter === "all" ? "active" : ""} onClick={() => setStatusFilter("all")}><span className="status-dot" />All<span className="filter-count">{projects.length}</span></button>
              {statusOrder.filter((key) => statusCounts[key] > 0).map((key) => <button key={key} className={statusFilter === key ? "active" : ""} onClick={() => setStatusFilter(key)}><span className={`status-dot ${key}`} />{statusFilterLabel[key]}<span className="filter-count">{statusCounts[key]}</span></button>)}
            </div>
            <div>
              <p className="nav-label">Controller</p>
              <button className={controllerFilter === "all" ? "active" : ""} onClick={() => setControllerFilter("all")}>All<span className="filter-count">{projects.length}</span></button>
              {controllerCounts.map(([controller, count]) => <button key={controller} className={controllerFilter === controller ? "active" : ""} onClick={() => setControllerFilter(controller)}>{controller}<span className="filter-count">{count}</span></button>)}
            </div>
          </aside>

          <div className="project-dash-main">
            <div className="project-dash-toolbar">
              <label className="project-dash-search"><Search size={14} /><input type="text" placeholder="Find a project…" value={search} onChange={(event) => setSearch(event.target.value)} /></label>
              <select value={sort} onChange={(event) => setSort(event.target.value as "updated" | "name")}>
                <option value="updated">Sort: Last activity</option>
                <option value="name">Sort: Name</option>
              </select>
            </div>

            {filtered.length === 0 ? <div className="panel"><div className="empty-state"><FolderGit2 size={24} /><h2>No matching projects</h2><p>Try a different filter or search term.</p></div></div>
              : <div className="panel project-dash-list">
                  {filtered.map((project) => {
                    const meta = statusMeta[project.analysis_status];
                    return <button key={project.id} className="project-dash-row" onClick={() => onOpenProject(project)}>
                      <span className="project-dash-icon">{project.name.slice(0, 2).toUpperCase()}</span>
                      <span className="project-dash-body">
                        <span className="project-dash-title"><strong>{project.name}</strong><span className={`status-label ${meta.dot}`}>{meta.label}</span></span>
                        <span className="project-dash-description">{project.description || "No description provided."}</span>
                        <span className="project-dash-meta">
                          <span className={`status-dot ${meta.dot}`} />{project.controller} · {project.logic_voltage} V logic
                          <span className="project-dash-spacer" />
                          Updated {date(project.updated_at_ms)}
                        </span>
                      </span>
                      <ChevronRight size={16} />
                    </button>;
                  })}
                </div>}
          </div>
        </div>}
  </div>;
}
