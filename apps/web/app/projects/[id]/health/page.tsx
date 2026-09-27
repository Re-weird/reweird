"use client";

import { useRouter } from "next/navigation";
import { DeviceHealthView } from "../../../device-health";
import { useAppState } from "@/lib/app-state";
import { legacyViewPath } from "@/lib/project-routes";

export default function DeviceHealthPage() {
  const router = useRouter();
  const { profile, currentProjectID } = useAppState();
  return (
    <DeviceHealthView
      key={profile?.id ?? "none"}
      profile={profile}
      onNavigate={(view) => router.push(legacyViewPath(currentProjectID, view))}
    />
  );
}
