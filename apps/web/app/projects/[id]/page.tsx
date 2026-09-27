"use client";

import { useRouter } from "next/navigation";
import { Workbench } from "../../workbench";
import { DemoPlaceholderNotice, LiveTelemetryUnavailable, ProjectLivePending } from "../../project-views";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function ProjectWorkbenchPage() {
  const router = useRouter();
  const { session, practiceSession, practiceSource, project, profile, probePlan, source, liveAvailable, liveError, refreshLive, scenarios, busy, runScenario, runOriginalDemo, currentProjectID, historyProjectID } = useAppState();
  const openPractice = () => router.push(projectPath(currentProjectID, "simulator"));
  // Built-in demo: show the simulated example as labelled placeholder data.
  if (!project && (!liveAvailable || !session)) return <><DemoPlaceholderNotice onPractice={openPractice} /><Workbench session={practiceSession} project={null} profile={profile} plan={probePlan} source={practiceSource} scenarios={scenarios} busy={busy} onRunScenario={runScenario} onBrowserDemo={() => runOriginalDemo("wiggle")} onNavigate={(tab) => router.push(projectPath(currentProjectID, tab))} historyProjectID={historyProjectID} /></>;
  if (!liveAvailable || !session) return <LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} />;
  if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan ?? project?.probe_plan ?? null} />;
  return <Workbench session={session} project={project} profile={profile} plan={probePlan ?? project?.probe_plan ?? null} source={source} scenarios={scenarios} busy={busy} onRunScenario={runScenario} onBrowserDemo={() => runOriginalDemo("wiggle")} onNavigate={(tab) => router.push(projectPath(currentProjectID, tab))} historyProjectID={historyProjectID} />;
}
