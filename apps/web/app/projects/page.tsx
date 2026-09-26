"use client";

import { useRouter } from "next/navigation";
import { ProjectDashboardView } from "../project-dashboard";
import { useAppState } from "@/lib/app-state";

export default function ProjectsPage() {
  const router = useRouter();
  const { setShowNewProject } = useAppState();
  return <ProjectDashboardView
    onOpenProject={(project) => router.push(`/projects/${project.id}`)}
    onNewProject={() => setShowNewProject(true)}
  />;
}
