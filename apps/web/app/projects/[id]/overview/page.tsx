"use client";

import { useRouter } from "next/navigation";
import { ProjectOverviewView } from "../../../project-overview";
import { useAppState } from "@/lib/app-state";
import { legacyViewPath } from "@/lib/project-routes";

export default function ProjectOverviewPage() {
  const router = useRouter();
  const { project, profile, probePlan, session, saveProfile, confirmProfile, syncRepository, reviseProfile, currentProjectID } = useAppState();
  return (
    <ProjectOverviewView
      project={project}
      profile={profile}
      plan={probePlan ?? project?.probe_plan ?? null}
      session={session}
      onSave={saveProfile}
      onConfirm={confirmProfile}
      onSync={syncRepository}
      onRevise={project ? reviseProfile : undefined}
      onNavigate={(view) => router.push(legacyViewPath(currentProjectID, view))}
    />
  );
}
