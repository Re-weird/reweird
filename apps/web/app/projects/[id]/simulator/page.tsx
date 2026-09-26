"use client";

import { SimulatorView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";

export default function SimulatorPage() {
  const { session, scenarios, selectedScenario, setSelectedScenario, runScenario, runTestAction, runOriginalDemo, busy } = useAppState();
  return <SimulatorView
    session={session}
    scenarios={scenarios}
    selected={selectedScenario}
    setSelected={setSelectedScenario}
    onRun={runScenario}
    onPlan={() => runTestAction("plan")}
    onDemoTest={() => runOriginalDemo("wiggle")}
    onDemoRepair={() => runOriginalDemo("repair")}
    busy={busy}
  />;
}
