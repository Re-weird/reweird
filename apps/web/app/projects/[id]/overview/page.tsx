"use client";

import { useRouter } from "next/navigation";
import { ProjectProfileView } from "../../../project-workflow";
import { useAppState } from "@/lib/app-state";
import { legacyViewPath } from "@/lib/project-routes";

export default function ProjectOverviewPage() {
  const router = useRouter();
  const { project, profile, probePlan, session, saveProfile, confirmProfile, currentProjectID } = useAppState();
  return (
    <ProjectProfileView
      project={project}
      profile={profile}
      plan={probePlan ?? project?.probe_plan ?? null}
      session={session}
      onSave={saveProfile}
      onConfirm={confirmProfile}
      onNavigate={(view) => router.push(legacyViewPath(currentProjectID, view))}
    />
  );
}
