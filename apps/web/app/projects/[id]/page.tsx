"use client";

import { useRouter } from "next/navigation";
import { Workbench } from "../../workbench";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function ProjectWorkbenchPage() {
  const router = useRouter();
  const { session, project, profile, source, currentProjectID, historyProjectID } = useAppState();
  return <Workbench session={session} project={project} profile={profile} source={source} onNavigate={(tab) => router.push(projectPath(currentProjectID, tab))} historyProjectID={historyProjectID} />;
}
