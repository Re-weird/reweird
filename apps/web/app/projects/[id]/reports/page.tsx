"use client";

import { HistoryReportView } from "../../../history-report";
import { useAppState } from "@/lib/app-state";

export default function ReportsPage() {
  const { historyProjectID } = useAppState();
  return <HistoryReportView mode="reports" projectID={historyProjectID} />;
}
