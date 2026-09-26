"use client";

import { useRouter } from "next/navigation";
import { DevicePassportView } from "../../../device-passport";
import { useAppState } from "@/lib/app-state";
import { legacyViewPath } from "@/lib/project-routes";

export default function DevicePassportPage() {
  const router = useRouter();
  const { profile, project, probePlan, session, currentProjectID } = useAppState();
  return (
    <DevicePassportView
      key={profile?.id ?? "none"}
      profile={profile}
      project={project}
      plan={probePlan ?? project?.probe_plan ?? null}
      session={session}
      onNavigate={(view) => router.push(legacyViewPath(currentProjectID, view))}
    />
  );
}
