"use client";

import { DiagnosisView, ProjectLivePending } from "../../../project-views";
import { useAppState } from "@/lib/app-state";

export default function DiagnosisPage() {
  const { project, profile, probePlan, session, source, runTestAction, busy } = useAppState();
  if (project && session.profile_id !== profile?.id) return <ProjectLivePending project={project} profile={profile} plan={probePlan} />;
  return <DiagnosisView session={session} source={source} onPlan={() => runTestAction("plan")} busy={busy} />;
}
