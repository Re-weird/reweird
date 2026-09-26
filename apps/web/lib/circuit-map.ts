import type { DemoSession, ProbeInstruction, ProbePlan, ProbeReading, ProfileComponent, ProfileConnection, ProjectProfile, RuleResult } from "@reweird/shared-types";

export interface CircuitMapConnection {
  connection: ProfileConnection;
  instruction?: ProbeInstruction;
  reading?: ProbeReading;
  rules: RuleResult[];
}

export interface CircuitMapGroup {
  key: string;
  name: string;
  component?: ProfileComponent;
  connections: CircuitMapConnection[];
}

export interface CircuitMapModel {
  groups: CircuitMapGroup[];
  hasMatchingCapture: boolean;
  captureKind?: "serial" | "simulator";
  planConnected: boolean;
}

function normalized(value: string): string {
  return value.trim().toLowerCase();
}

function componentFor(connection: ProfileConnection, components: ProfileComponent[]): ProfileComponent | undefined {
  if (connection.component_id) {
    const byID = components.find((component) => component.id === connection.component_id);
    if (byID) return byID;
  }
  return components.find((component) => normalized(component.name) === normalized(connection.component_name));
}

export function buildCircuitMap(profile: ProjectProfile, plan: ProbePlan | null, session: DemoSession | null, demoMode = false): CircuitMapModel {
  const currentPlan = profile.confirmed && plan?.profile_id === profile.id && (!profile.project_id || plan.project_id === profile.project_id) ? plan : null;
  const planConnected = Boolean(currentPlan?.connected);
  const hasMatchingCapture = Boolean(
    profile.confirmed &&
    (demoMode || planConnected) &&
    session?.profile_id === profile.id &&
    session.raw_telemetry?.profile_id === profile.id &&
    session.analysis?.profile_id === profile.id &&
    (session.measurement_id ?? 0) > 0,
  );
  const capture = hasMatchingCapture ? session : null;
  const instructions = (currentPlan?.instructions ?? []).filter((instruction) => instruction.probe !== "GND");
  const usedInstructions = new Set<number>();
  const groups: CircuitMapGroup[] = profile.components.map((component) => ({ key: `component:${component.id}`, name: component.name, component, connections: [] }));

  for (const connection of profile.connections ?? []) {
    const component = componentFor(connection, profile.components);
    const key = component ? `component:${component.id}` : `unmatched:${normalized(connection.component_name)}`;
    let group = groups.find((item) => item.key === key);
    if (!group) {
      group = { key, name: connection.component_name || "Unspecified component", connections: [] };
      groups.push(group);
    }
    const instructionIndex = instructions.findIndex((item, index) =>
      !usedInstructions.has(index) && item.role === connection.role && item.target === connection.target,
    );
    const instruction = instructionIndex >= 0 ? instructions[instructionIndex] : undefined;
    if (instructionIndex >= 0) usedInstructions.add(instructionIndex);
    const reading = instruction && capture ? capture.probes.find((item) => item.probe === instruction.probe) : undefined;
    group.connections.push({
      connection,
      instruction,
      reading,
      rules: instruction && capture ? capture.evidence.rule_results.filter((rule) => rule.probe === instruction.probe) : [],
    });
  }

  return {
    groups,
    hasMatchingCapture,
    captureKind: capture ? capture.telemetry_mode === "serial" ? "serial" : "simulator" : undefined,
    planConnected,
  };
}
