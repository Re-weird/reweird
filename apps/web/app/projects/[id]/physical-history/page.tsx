"use client";

import { PhysicalHistoryView } from "../../../physical-history";
import { useAppState } from "@/lib/app-state";

export default function PhysicalHistoryPage() {
  const { project } = useAppState();
  return <PhysicalHistoryView project={project} />;
}
