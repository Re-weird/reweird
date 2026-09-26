"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useParams } from "next/navigation";
import { Box, Cable, CheckCircle2, Cpu, FileBarChart, LayoutDashboard, Microscope, RefreshCw, TestTube2 } from "lucide-react";
import { useAppState } from "@/lib/app-state";
import { DEMO_PROJECT_ID, projectTabs, type ProjectTabID } from "@/lib/project-routes";

const icons: Record<ProjectTabID, typeof Box> = {
  workbench: LayoutDashboard,
  overview: Box,
  "probe-setup": Cable,
  simulator: TestTube2,
  diagnosis: Microscope,
  "next-test": TestTube2,
  verify: CheckCircle2,
  history: RefreshCw,
  reports: FileBarChart,
  computer: Cpu,
};

export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  const params = useParams<{ id: string }>();
  const pathname = usePathname();
  const { loadProject, project, session } = useAppState();
  const id = params.id;

  useEffect(() => {
    loadProject(id);
    // Only re-run when the URL's project id changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const base = `/projects/${id}`;
  const projectName = id === DEMO_PROJECT_ID ? "Built-in demo" : project?.name ?? "Loading…";

  return (
    <div className="project-shell">
      <div className="project-shell-heading">
        <span className="kicker">Project</span>
        <h1>{projectName}</h1>
      </div>
      <nav className="project-tabs" aria-label="Project navigation">
        {projectTabs.map((tab) => {
          const Icon = icons[tab.id];
          const href = tab.segment ? `${base}/${tab.segment}` : base;
          const active = pathname === href;
          return <Link key={tab.id} href={href} className={active ? "active" : ""} aria-current={active ? "page" : undefined}>
            <Icon size={15} /><span>{tab.label}</span>
            {tab.id === "diagnosis" && session.stage !== "verify" && <i />}
          </Link>;
        })}
      </nav>
      {children}
    </div>
  );
}
