"use client";

import { DiagnosisView, LiveTelemetryUnavailable, ProjectLivePending } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { useRouter } from "next/navigation";
import { projectPath } from "@/lib/project-routes";

export default function DiagnosisPage() {
  const router = useRouter();
  const { project, profile, probePlan, session, source, liveAvailable, liveError, refreshLive, currentProjectID, runTestAction, busy } = useAppState();
  if (!liveAvailable || !session) return <LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} />;
  if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan} />;
  return <DiagnosisView session={session} source={source} onPlan={() => runTestAction("plan")} busy={busy} />;
}
