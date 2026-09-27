"use client";

import { DemoPlaceholderNotice, DiagnosisView, LiveTelemetryUnavailable, ProjectLivePending } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { useRouter } from "next/navigation";
import { projectPath } from "@/lib/project-routes";

export default function DiagnosisPage() {
  const router = useRouter();
  const { project, profile, probePlan, session, source, practiceSession, practiceSource, liveAvailable, liveError, refreshLive, currentProjectID, runTestAction, busy } = useAppState();
  const openPractice = () => router.push(projectPath(currentProjectID, "simulator"));
  // The built-in demo has no physical circuit, so it shows the simulated
  // example as clearly-labelled placeholder data instead of an empty state.
  if (!project && (!liveAvailable || !session)) return <><DemoPlaceholderNotice onPractice={openPractice} /><DiagnosisView session={practiceSession} source={practiceSource} onPlan={openPractice} busy={busy} /></>;
  if (!liveAvailable || !session) return <LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} />;
  if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan} />;
  return <DiagnosisView session={session} source={source} onPlan={() => runTestAction("plan")} busy={busy} />;
}
