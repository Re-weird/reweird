"use client";

import { useEffect, useState } from "react";
import { ChevronRight, FolderGit2, Upload } from "lucide-react";
import type { Project } from "@reweird/shared-types";
import { projectApi } from "@/lib/api";

const date = (value?: number) => value ? new Date(value).toLocaleString() : "—";

const statusMeta: Record<Project["analysis_status"], { dot: "stable" | "active" | "intermittent" | "idle"; label: string }> = {
  CONFIRMED: { dot: "stable", label: "Profile confirmed" },
  DRAFT_READY: { dot: "active", label: "Draft ready for review" },
  PROCESSING: { dot: "active", label: "Analyzing" },
  PENDING: { dot: "idle", label: "Awaiting analysis" },
  FAILED: { dot: "intermittent", label: "Analysis failed" },
};

export function ProjectDashboardView({ onOpenProject, onNewProject }: { onOpenProject: (project: Project) => void; onNewProject: () => void }) {
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    projectApi.listProjects().then((items) => { if (live) setProjects(items ?? []); })
      .catch((cause) => { if (live) { setError(cause instanceof Error ? cause.message : "Projects are unavailable."); setProjects([]); } });
    return () => { live = false; };
  }, []);

  return <div className="project-dashboard">
    <section className="page-heading">
      <div><span className="kicker">Workspace</span><h1>Your projects</h1><p>Every project analyzed on this ReWeird instance. Open one to review its evidence, or upload a new project to start a profile.</p></div>
      <button className="primary" onClick={onNewProject}><Upload size={15} /> Upload a project</button>
    </section>

    {error && <div className="form-error page-error">{error}</div>}

    {projects === null ? <div className="panel"><div className="empty-state"><FolderGit2 size={24} /><h2>Loading projects…</h2></div></div>
      : projects.length === 0 ? <div className="panel"><div className="empty-state"><FolderGit2 size={24} /><h2>No projects yet</h2><p>Upload a project's code and an optional hardware photo to build its first profile.</p><button className="primary" onClick={onNewProject}><Upload size={15} /> Upload a project</button></div></div>
      : <div className="panel project-dash-list">
          {projects.map((project) => {
            const meta = statusMeta[project.analysis_status];
            return <button key={project.id} className="project-dash-row" onClick={() => onOpenProject(project)}>
              <span className={`status-dot ${meta.dot}`} />
              <span className="project-dash-name"><strong>{project.name}</strong><small>{project.controller} · {project.logic_voltage} V logic</small></span>
              <span className={`status-label ${meta.dot}`}>{meta.label}</span>
              <span className="project-dash-updated">Updated {date(project.updated_at_ms)}</span>
              <ChevronRight size={16} />
            </button>;
          })}
        </div>}
  </div>;
}
