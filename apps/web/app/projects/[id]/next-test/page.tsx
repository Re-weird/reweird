"use client";

import { GuidedTestView } from "../../../guided-test";
import { LiveTelemetryUnavailable } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { useRouter } from "next/navigation";
import { projectPath } from "@/lib/project-routes";
import { PatchStatus } from "../../../patch-status";

export default function NextTestPage() {
  const router = useRouter();
  const { workflow, recommendation, busy, testError, runTestAction, recordUserAction, liveAvailable, liveError, refreshLive, currentProjectID } = useAppState();
  if (!liveAvailable) return <><PatchStatus /><LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} /></>;
  return <><PatchStatus /><GuidedTestView
    workflow={workflow}
    recommendation={recommendation}
    busy={busy}
    error={testError}
    onPlan={() => runTestAction("plan")}
    onStart={() => runTestAction("start")}
    onCapture={() => runTestAction("capture")}
    onRemeasure={() => runTestAction("remeasure")}
    onCancel={() => runTestAction("cancel")}
    onRecordAction={recordUserAction}
  /></>;
}
