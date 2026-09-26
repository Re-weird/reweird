"use client";

import { GuidedTestView } from "../../../guided-test";
import { LegacyDemoVerifyView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";

export default function VerifyPage() {
  const { session, legacyVerify, busy, testError, workflow, recommendation, runOriginalDemo, runTestAction, recordUserAction } = useAppState();
  if (legacyVerify) return <LegacyDemoVerifyView session={session} onReset={() => runOriginalDemo("reset")} busy={busy} />;
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
  />;
}
