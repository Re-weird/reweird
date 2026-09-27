"use client";

import { useRouter } from "next/navigation";
import { ProbePlanView } from "../../../project-workflow";
import { DemoProbePlanView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function ProbeSetupPage() {
  const router = useRouter();
  const { project, profile, session, probePlan, confirmConnections, currentProjectID } = useAppState();
  if (project) return <ProbePlanView project={project} profile={profile} session={session} plan={probePlan ?? project.probe_plan ?? null} onConnected={confirmConnections} />;
  return <DemoProbePlanView plan={probePlan} onContinue={() => router.push(projectPath(currentProjectID, "simulator"))} />;
}
