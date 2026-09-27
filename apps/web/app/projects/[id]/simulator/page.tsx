"use client";

import { SimulatorView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";

export default function SimulatorPage() {
  const { practiceSession, source, scenarios, selectedScenario, setSelectedScenario, scenarioError, mysteryPending, revealMystery, runScenario, runOriginalDemo, busy } = useAppState();
  return <SimulatorView
    session={practiceSession}
    source={source}
    scenarios={scenarios}
    selected={selectedScenario}
    error={scenarioError}
    setSelected={setSelectedScenario}
    mysteryPending={mysteryPending}
    onRevealMystery={revealMystery}
    onRun={() => runScenario(selectedScenario)}
    onDemoTest={() => runOriginalDemo("wiggle")}
    onDemoRepair={() => runOriginalDemo("repair")}
    busy={busy}
  />;
}
