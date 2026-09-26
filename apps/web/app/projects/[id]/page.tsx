"use client";

import { useRouter } from "next/navigation";
import { Workbench } from "../../workbench";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function ProjectWorkbenchPage() {
  const router = useRouter();
  const { session, project, profile, probePlan, source, scenarios, busy, runScenario, runOriginalDemo, currentProjectID, historyProjectID } = useAppState();
  return <Workbench session={session} project={project} profile={profile} plan={probePlan} source={source} scenarios={scenarios} busy={busy} onRunScenario={runScenario} onBrowserDemo={() => runOriginalDemo("wiggle")} onNavigate={(tab) => router.push(projectPath(currentProjectID, tab))} historyProjectID={historyProjectID} />;
}
