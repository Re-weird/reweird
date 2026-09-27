"use client";

import { useRouter } from "next/navigation";
import { ProjectProfileView } from "../../../project-workflow";
import { RepositoryStatus } from "../../../repository-status";
import { useAppState } from "@/lib/app-state";
import { legacyViewPath } from "@/lib/project-routes";

export default function ProjectOverviewPage() {
  const router = useRouter();
  const { project, profile, probePlan, session, saveProfile, confirmProfile, syncRepository, currentProjectID } = useAppState();
  return (
    <>
      {project?.repository && <RepositoryStatus repository={project.repository} onSync={syncRepository} />}
      <ProjectProfileView
        project={project}
        profile={profile}
        plan={probePlan ?? project?.probe_plan ?? null}
        session={session}
        onSave={saveProfile}
        onConfirm={confirmProfile}
        onNavigate={(view) => router.push(legacyViewPath(currentProjectID, view))}
      />
    </>
  );
}
