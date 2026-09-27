"use client";

import { useRouter } from "next/navigation";
import { GuidedTestView } from "../../../guided-test";
import { LegacyDemoVerifyView } from "../../../project-views";
import { LiveTelemetryUnavailable } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { projectPath } from "@/lib/project-routes";

export default function VerifyPage() {
  const router = useRouter();
  const { session, legacyVerify, busy, testError, workflow, recommendation, runOriginalDemo, runTestAction, recordUserAction, currentProjectID, liveAvailable, liveError, refreshLive } = useAppState();
  if (!liveAvailable || !session) return <LiveTelemetryUnavailable error={liveError} onRetry={() => void refreshLive()} onPractice={() => router.push(projectPath(currentProjectID, "simulator"))} />;
  if (legacyVerify && session.telemetry_mode !== "serial") return <LegacyDemoVerifyView session={session} onReset={() => runOriginalDemo("reset")} busy={busy} />;
  return <GuidedTestView
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
    onHealth={() => router.push(projectPath(currentProjectID, "health"))}
  />;
}
