"use client";

import { SimulatorView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";

export default function SimulatorPage() {
  const { session, source, scenarios, selectedScenario, setSelectedScenario, mysteryPending, revealMystery, runScenario, runTestAction, runOriginalDemo, busy } = useAppState();
  return <SimulatorView
    session={session}
    source={source}
    scenarios={scenarios}
    selected={selectedScenario}
    setSelected={setSelectedScenario}
    mysteryPending={mysteryPending}
    onRevealMystery={revealMystery}
    onRun={runScenario}
    onPlan={() => runTestAction("plan")}
    onDemoTest={() => runOriginalDemo("wiggle")}
    onDemoRepair={() => runOriginalDemo("repair")}
    busy={busy}
  />;
}
