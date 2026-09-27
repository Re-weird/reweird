"use client";

import { SimulatorView } from "../../../project-views";
import { useAppState } from "@/lib/app-state";
import { useRouter } from "next/navigation";
import { projectPath, DEMO_PROJECT_ID } from "@/lib/project-routes";

export default function SimulatorPage() {
  const router = useRouter();
  const { session, source, scenarios, selectedScenario, setSelectedScenario, scenarioError, mysteryPending, revealMystery, runScenario, runTestAction, runOriginalDemo, busy } = useAppState();
  return <SimulatorView
    session={session}
    source={source}
    scenarios={scenarios}
    selected={selectedScenario}
    error={scenarioError}
    setSelected={setSelectedScenario}
    mysteryPending={mysteryPending}
    onRevealMystery={revealMystery}
    onRun={() => runScenario(selectedScenario)}
    onDiagnose={() => router.push(projectPath(DEMO_PROJECT_ID, "diagnosis"))}
    onPlan={() => runTestAction("plan")}
    onDemoTest={() => runOriginalDemo("wiggle")}
    onDemoRepair={() => runOriginalDemo("repair")}
    busy={busy}
  />;
}
