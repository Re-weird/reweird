"use client";

import { HistoryReportView } from "../../../history-report";
import { useAppState } from "@/lib/app-state";

export default function HistoryPage() {
  const { historyProjectID } = useAppState();
  return <HistoryReportView mode="history" projectID={historyProjectID} />;
}
