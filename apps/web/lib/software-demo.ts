import type { DemoSession, DiagnosticWorkflow, HistorySummary, MeasurementWindow, ProbePlan, ProjectProfile, TestRecommendation } from "@reweird/shared-types";
import { makeDemoProfile } from "./demo";
import { BASELINE, FAULTS, FAULT_ORDER, TESTS, checks, initialState, reduce, verifies, type FaultID, type Snapshot, type TestID } from "./judge-demo";

export type ScenarioID = FaultID | "healthy";
export const scenarios = [
  { id: "healthy", name: "Healthy baseline", description: "Stable 5 V rail, 10 µs trigger, and consistent ECHO replies.", expected_finding: "All checks pass." },
  ...FAULT_ORDER.map(id => ({ id, name: FAULTS[id].label, description: FAULTS[id].blurb, expected_finding: FAULTS[id].cause })),
];
export const testFor: Record<FaultID, TestID> = { "loose-echo": "wiggle", "unstable-power": "rail-load", "missing-echo": "continuity", "timing-drift": "trigger-timing" };
export const snapshotFor = (id: ScenarioID): Snapshot => id === "healthy" ? BASELINE : FAULTS[id].symptom;
export function softwareProfile(): ProjectProfile {
  const profile = makeDemoProfile();
  // The HC-SR04's 40 kHz acoustic carrier is not the trigger repetition rate.
  const trig = profile.connections?.find(item => item.role === "TRIG");
  if (trig) trig.expected.nominal_frequency_hz = 28.4;
  profile.probes[1].expected.nominal_frequency_hz = 28.4;
  return profile;
}
export function softwarePlan(): ProbePlan {
  return {
    project_id: "ultrasonic-demo", profile_id: "ultrasonic-demo", connected: true, generated_at_ms: 0,
    instructions: [
      { probe: "GND", role: "REFERENCE", target: "Common ESP32 / HC-SR04 ground", expected: "0 V reference", signal_type: "ground", safe_warning: "Demo only; no physical connection is made." },
      { probe: "P1", role: "POWER", target: "HC-SR04 5 V supply", expected: "4.75–5.25 V", signal_type: "analog", safe_warning: "Real hardware requires a verified voltage divider before an ESP32 ADC." },
      { probe: "P2", role: "TRIG", target: "ESP32 GPIO5 → HC-SR04 TRIG", expected: "10 µs pulse at 28.4 Hz", signal_type: "pulse", safe_warning: "Input-only monitoring." },
      { probe: "P3", role: "ECHO", target: "HC-SR04 ECHO → ESP32 GPIO18", expected: "28.4 replies/s, no dropouts", signal_type: "pulse", safe_warning: "Real 5 V ECHO requires a verified divider to 3.3 V." },
    ],
  };
}

export function softwareSession(s: Snapshot, scenario: ScenarioID, phase: "diagnose" | "test" | "verify" = "diagnose", sequence = 1): DemoSession {
  const rules = checks(s);
  const healthy = verifies(s);
  const target = rules.find(rule => rule.status === "fail")?.probe ?? "P3";
  const roles = { P1: "POWER", P2: "TRIG", P3: "ECHO" };
  const now = Date.now();
  const probes: DemoSession["probes"] = [
    { probe: "P1", role: "POWER", value: s.power.mean_v, unit: "V", status: s.power.min_v >= 4.75 ? "stable" : "intermittent", dropouts: 0, samples: s.power.samples },
    { probe: "P2", role: "TRIG", value: s.trig.rate_hz, unit: "Hz", status: s.trig.width_us >= 10 ? "active" : "intermittent", dropouts: 0, samples: s.trig.samples },
    { probe: "P3", role: "ECHO", value: s.echo.rate_hz, unit: "pulses/s", status: healthy ? "stable" : "intermittent", dropouts: s.echo.dropouts_per_min, samples: s.echo.samples },
  ];
  const headline = healthy ? (phase === "verify" ? "Issue resolved" : "Signals match the healthy baseline") : phase === "test" ? "Follow-up test captured" : target === "P1" ? "Power rail falls below specification" : target === "P2" ? "Trigger pulse is shorter than expected" : s.echo.rate_hz === 0 ? "ECHO response is missing" : "Intermittent ECHO activity";
  const raw: DemoSession["raw_telemetry"] = {
    schema_version: 2, device_id: "browser-simulated-esp32", profile_id: "ultrasonic-demo", captured_at_ms: now, uptime_ms: sequence * 60000, window_ms: 60000, sequence,
    samples: [
      { probe: "P1", mode: "analog", analog_mv: s.power.samples.map(v => Math.round(v * 500)) },
      { probe: "P2", mode: "pulse", periods_us: s.trig.samples.map(v => Math.round(1e6 / v)), high_pulse_widths_us: s.trig.samples.map(() => s.trig.width_us), activity_counts: s.trig.samples.map(v => Math.round(v * 3.75)) },
      { probe: "P3", mode: "pulse", periods_us: s.echo.samples.filter(v => v > 0).map(v => Math.round(1e6 / v)), activity_counts: s.echo.samples.map(v => Math.round(v * 3.75)) },
    ],
  };
  return {
    id: "browser-project-demo", project_name: "Ultrasonic Distance Sensor", stage: phase, hardware_connected: false,
    telemetry_mode: "browser", profile_id: "ultrasonic-demo", scenario_id: scenario, measurement_id: sequence, raw_telemetry: raw, probes,
    analysis: { schema_version: 2, device_id: raw.device_id, profile_id: raw.profile_id, captured_at_ms: now, window_ms: 60000,
      probes: probes.map((p, i) => ({ probe: p.probe, role: p.role, mode: i === 0 ? "analog" : "pulse", digital_transitions: i ? Math.round((p.value ?? 0) * 120) : 0, pulse_count: i ? Math.round((p.value ?? 0) * 60) : 0, dropout_events: p.dropouts, missing_expected_activity: p.value === 0, stable: p.status !== "intermittent", ...(i === 0 ? { average_voltage: s.power.mean_v, minimum_voltage: s.power.min_v, maximum_voltage: Math.max(...s.power.samples), voltage_variation: Math.max(...s.power.samples) - s.power.min_v } : { frequency_hz: p.value ?? 0, average_pulse_width_us: i === 1 ? s.trig.width_us : undefined }) })) },
    evidence: { probe: target, role: roles[target], expected: { signal: "stable power, valid trigger, reliable echo", min_rail_voltage: 4.75, min_trigger_width_us: 10, echo_replies_per_second: 28.4 },
      observed: { min_rail_voltage: s.power.min_v, trigger_width_us: s.trig.width_us, echo_replies_per_second: s.echo.rate_hz, dropouts_per_minute: s.echo.dropouts_per_min },
      baseline: { status: "KNOWN_GOOD_CAPTURE", trusted: true, source: "SIMULATED", dropouts_per_minute: 0, average_pulses_per_second: 28.4 },
      rule_results: rules.map(rule => ({ ...rule, provenance: "DERIVED" })) },
    diagnosis: { headline, summary: healthy ? "All simulated readings pass the same deterministic checks and match the simulated healthy baseline." : rules.filter(rule => rule.status !== "pass").map(rule => rule.message).join(". "),
      possible_causes: healthy ? [] : target === "P1" ? ["Weak supply", "Excess load", "Supply connection resistance"] : target === "P2" ? ["Incorrect firmware pulse width", "Timer configuration"] : ["Loose or disconnected ECHO wire", "Sensor fault", "Missing response"],
      confidence: healthy ? .98 : phase === "test" ? .92 : .68,
      next_test: healthy ? "Capture again to confirm continued stability." : TESTS[testFor[scenario === "healthy" ? "loose-echo" : scenario]].instruction },
    before: { dropouts_per_minute: snapshotFor(scenario).echo.dropouts_per_min, stability: "simulated initial capture" },
    after: phase === "verify" ? { dropouts_per_minute: s.echo.dropouts_per_min, stability: healthy ? "stable" : "unresolved" } : undefined,
    timeline: [ { id: "detect", label: "Detect", detail: "Capture simulated signals", time: "01", complete: true }, { id: "diagnose", label: "Diagnose", detail: "Compare evidence and rules", time: "02", complete: true }, { id: "test", label: "Test", detail: "Capture a follow-up test", time: "03", complete: phase !== "diagnose" }, { id: "verify", label: "Verify", detail: "Compare before and after repair", time: "04", complete: phase === "verify" && healthy } ],
  };
}

export function testSnapshot(id: ScenarioID): { snapshot: Snapshot; finding: string } {
  if (id === "healthy") return { snapshot: BASELINE, finding: "A repeated simulated capture still passes every check." };
  let state = reduce(initialState, { type: "start", now: 1 });
  state = reduce(state, { type: "make-weird", fault: id });
  state = reduce(state, { type: "run-test", test: testFor[id] });
  return { snapshot: state.tests[0].during, finding: state.tests[0].finding };
}
export function recommendationFor(scenario: ScenarioID): TestRecommendation {
  const type = scenario === "unstable-power" ? "POWER_RAIL_STABILITY" : scenario === "timing-drift" ? "FREQUENCY_TIMING" : scenario === "missing-echo" ? "SIGNAL_ACTIVITY" : scenario === "healthy" ? "REMEASURE" : "MOVEMENT_CORRELATION";
  return { id: "browser-recommendation", session_id: "browser-project-demo", test_type: type, target_probes: [scenario === "unstable-power" ? "P1" : scenario === "timing-drift" ? "P2" : "P3"], reason: scenario === "healthy" ? "Repeat the healthy capture." : TESTS[testFor[scenario]].instruction, duration_seconds: 60, requires_user_action: true, requires_patch: false };
}
export function planWorkflow(scenario: ScenarioID): DiagnosticWorkflow {
  const recommendation = recommendationFor(scenario);
  return { id: `simulated-${Date.now()}`, session_id: "browser-project-demo", project_id: "ultrasonic-demo", profile_id: "ultrasonic-demo", profile_version: 1, scenario_id: scenario, status: "PLANNED", created_at_ms: Date.now(), updated_at_ms: Date.now(), user_actions: [],
    plan: { id: "browser-guided-plan", recommendation, title: scenario === "healthy" ? "Repeat baseline capture" : TESTS[testFor[scenario]].label,
      instructions: ["Capture the simulated before window.", "Use the capture button to simulate the guided action; do not touch physical hardware.", "Compare the resulting evidence, then simulate a correction and verify."], monitoring: recommendation.target_probes, metrics: ["rail voltage", "trigger pulse width", "echo rate", "dropouts"], criteria: "Compare before, during, and after using the same deterministic rules. All data is simulated.", window_ms: 60000, requires_patch: false } };
}
export function measurement(session: DemoSession): MeasurementWindow {
  return { id: session.measurement_id!, profile_id: "ultrasonic-demo", source: "SIMULATED", device_id: "browser-simulated-esp32", sequence: session.measurement_id!, captured_at_ms: Date.now(), ingested_at_ms: Date.now(), raw: session.raw_telemetry!, analysis: session.analysis! };
}
export function summaryFor(workflow: DiagnosticWorkflow): HistorySummary {
  return { id: workflow.id, project_id: workflow.project_id, project_name: "Ultrasonic Distance Sensor", session_id: workflow.session_id, profile_id: workflow.profile_id, profile_version: 1, telemetry_source: "SIMULATED", original_problem: workflow.plan.title, status: workflow.status === "RESOLVED" ? "RESOLVED" : workflow.status === "CANCELLED" ? "CANCELLED" : "TESTING", started_at_ms: workflow.created_at_ms };
}
