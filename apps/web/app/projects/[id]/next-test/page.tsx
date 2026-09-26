"use client";

import { GuidedTestView } from "../../../guided-test";
import { useAppState } from "@/lib/app-state";

export default function NextTestPage() {
  const { workflow, recommendation, busy, testError, runTestAction, recordUserAction } = useAppState();
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
