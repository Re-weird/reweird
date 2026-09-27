"use client";

import { useRouter } from "next/navigation";
import { Workbench } from "../../workbench";
import { LiveTelemetryUnavailable, ProjectLivePending } from "../../project-views";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function ProjectWorkbenchPage() {
  const router = useRouter();
  const { session, project, profile, probePlan, source, liveAvailable, liveError, refreshLive, scenarios, busy, runScenario, runOriginalDemo, currentProjectID, historyProjectID } = useAppState();
  if (!liveAvailable || !session) return <LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} />;
  if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan ?? project?.probe_plan ?? null} />;
  return <Workbench session={session} project={project} profile={profile} plan={probePlan ?? project?.probe_plan ?? null} source={source} scenarios={scenarios} busy={busy} onRunScenario={runScenario} onBrowserDemo={() => runOriginalDemo("wiggle")} onNavigate={(tab) => router.push(projectPath(currentProjectID, tab))} historyProjectID={historyProjectID} />;
}
