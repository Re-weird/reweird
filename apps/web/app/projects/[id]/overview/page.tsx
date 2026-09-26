"use client";

import { ProjectProfileView } from "../../../project-workflow";
import { useAppState } from "@/lib/app-state";

export default function ProjectOverviewPage() {
  const { project, profile, saveProfile, confirmProfile } = useAppState();
  return <ProjectProfileView project={project} profile={profile} onSave={saveProfile} onConfirm={confirmProfile} />;
}
