"use client";

import { ProjectDashboardView } from "../project-dashboard";
import { useAppState } from "@/lib/app-state";

export default function ProjectsPage() {
  const { setShowNewProject } = useAppState();
  return <ProjectDashboardView onNewProject={() => setShowNewProject(true)} />;
}
