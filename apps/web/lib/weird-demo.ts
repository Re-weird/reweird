import type { DemoSession, DiagnosticWorkflow, ProbePlan, ProjectProfile, SimulatorScenario, TestRecommendation } from "@reweird/shared-types";

export const weirdChoices = [
  { label: "Loose connection", scenario: "intermittent-connection" },
  { label: "Unstable power", scenario: "unstable-power" },
  { label: "Missing signal", scenario: "dead-signal" },
  { label: "Timing problem", scenario: "timing-drift" },
] as const;

export interface TelemetryStatus {
	transport?: "direct" | "usb_bridge";
  mode: string;
  connected: boolean;
  error?: string;
  device_id?: string;
  profile_id?: string;
  captured_at_ms?: number;
}

// A connected serial port is not proof that the diagnostic engine accepted
// its frame. In particular, a profile/mode mismatch makes /session return
// 503 while /telemetry/status still reports connected=true.
export function isCompatibleSerialSession(status: TelemetryStatus | null, session: DemoSession | null): session is DemoSession {
  return Boolean(status?.connected && status.mode === "serial" && status.device_id && status.profile_id &&
    session?.telemetry_mode === "serial" && session.measurement_id && session.raw_telemetry &&
    session.profile_id === status.profile_id && session.raw_telemetry.profile_id === status.profile_id &&
    session.raw_telemetry.device_id === status.device_id);
}

export function sessionForView(practice: boolean, live: DemoSession | null, simulator: DemoSession): DemoSession | null {
  return practice ? simulator : live;
}

export function serialWorkflowMatches(workflow: DiagnosticWorkflow | null, session: DemoSession | null): boolean {
  if (!workflow || !session || workflow.profile_id !== session.profile_id || workflow.scenario_id) return false;
  return [workflow.baseline, workflow.during, workflow.after].every((window) =>
    !window || (window.source === "serial" && window.device_id === session.raw_telemetry?.device_id));
}

export function serialRecommendationMatches(recommendation: TestRecommendation | null, session: DemoSession | null): boolean {
  return Boolean(recommendation && session?.telemetry_mode === "serial" && recommendation.session_id === session.id);
}

export function availableWeirdChoices(scenarios: SimulatorScenario[]) {
  return weirdChoices.filter((choice) => scenarios.some((scenario) => scenario.id === choice.scenario));
}

export function mysteryScenario(scenarios: SimulatorScenario[], random = Math.random): string | null {
  const choices = availableWeirdChoices(scenarios);
  if (!choices.length) return null;
  return choices[Math.min(choices.length - 1, Math.floor(random() * choices.length))].scenario;
}

export function telemetryLabel(session: DemoSession, source: "api" | "browser"): string {
  if (source === "browser") return "BROWSER SIMULATOR";
  return session.telemetry_mode === "serial" ? "REAL SERIAL" : "API SIMULATOR";
}

export function realBreakReady(session: DemoSession, source: "api" | "browser", profile: ProjectProfile | null, plan: ProbePlan | null): boolean {
  return source === "api" && session.telemetry_mode === "serial" && session.hardware_connected === true &&
    Boolean(profile?.confirmed && plan?.connected && plan.profile_id === profile.id && session.profile_id === profile.id &&
      session.raw_telemetry?.profile_id === profile.id && session.measurement_id &&
      profile.probes?.some((probe) => /echo/i.test(probe.role)));
}

export function verifyHeadline(workflow: DiagnosticWorkflow | null): string | null {
  if (!workflow?.verification) return null;
  return workflow.verification.status === "RESOLVED" && workflow.status === "RESOLVED"
    ? "NOT WEIRD ANYMORE."
    : "STILL WEIRD.";
}

export function testHeadline(workflow: DiagnosticWorkflow | null): string | null {
  const result = workflow?.result?.result;
  if (!result) return null;
  if (["POSITIVE_CORRELATION", "SHARED_FAILURE_PATTERN", "RAIL_OUTSIDE_TOLERANCE", "EXPECTED_ACTIVITY_MISSING", "TIMING_OUTSIDE_SPECIFICATION", "BASELINE_DEVIATION"].includes(result)) return "WE FOUND SOMETHING.";
  if (result === "INCONCLUSIVE") return "STILL WEIRD.";
  return "THAT WASN’T IT.";
}
