import type { DemoSession, ProbeReading, SessionStage } from "@reweird/shared-types";

const powerSamples = [5.01, 5.0, 5.02, 5.01, 5.01, 5.0, 5.02, 5.01, 5.0, 5.01, 5.02, 5.01];
const trigSamples = [39.9, 40.1, 40, 40.2, 39.9, 40, 40.1, 40, 39.8, 40.1, 40, 40.2];
const echoFault = [29, 28, 29, 0, 28, 29, 12, 0, 28, 29, 0, 27];
const echoWiggle = [28, 0, 14, 0, 0, 27, 0, 10, 0, 0, 21, 0];
const echoHealthy = [28, 29, 28, 29, 28, 29, 28, 28, 29, 28, 29, 28];

function probes(stage: SessionStage): ProbeReading[] {
  const verified = stage === "verify";
  const wiggling = stage === "test" || stage === "repair";
  return [
    { probe: "P1", role: "POWER", value: 5.01, unit: "V", status: "stable", dropouts: 0, samples: powerSamples },
    { probe: "P2", role: "TRIG", value: 40, unit: "kHz", status: "active", dropouts: 0, samples: trigSamples },
    {
      probe: "P3",
      role: "ECHO",
      value: verified ? 28.4 : 19.7,
      unit: "pulses/s",
      status: verified ? "stable" : "intermittent",
      dropouts: verified ? 0 : wiggling ? 27 : 12,
      samples: verified ? echoHealthy : wiggling ? echoWiggle : echoFault,
    },
    { probe: "P4", role: "UNASSIGNED", value: null, unit: "", status: "idle", dropouts: 0, samples: [] },
  ];
}

export function makeDemoSession(stage: SessionStage = "diagnose"): DemoSession {
  const verified = stage === "verify";
  const wiggling = stage === "test" || stage === "repair";
  const dropoutCount = verified ? 0 : wiggling ? 27 : 12;

  return {
    id: "demo-ultrasonic-001",
    project_name: "Ultrasonic Distance Sensor",
    stage,
    hardware_connected: true,
    probes: probes(stage),
    evidence: {
      probe: "P3",
      role: "ECHO",
      expected: { signal: "return pulse", voltage: "3.3V logic", dropouts_per_minute: 0 },
      observed: {
        pulse_detected: true,
        dropouts_per_minute: dropoutCount,
        rail_voltage_stable: true,
        other_signals_active: true,
        movement_correlation: wiggling,
      },
      baseline: { dropouts_per_minute: 0, average_pulses_per_second: 28.4 },
      rule_results: [
        { id: "power-rail", status: "pass", message: "Power rail is stable at 5.01 V" },
        { id: "trigger-active", status: "pass", message: "TRIG activity is present at 40 kHz" },
        {
          id: "echo-dropouts",
          status: verified ? "pass" : "fail",
          message: verified ? "ECHO is stable after repair" : `${dropoutCount} unexpected ECHO dropouts detected`,
        },
        {
          id: "movement-correlation",
          status: wiggling ? "fail" : verified ? "pass" : "warn",
          message: wiggling
            ? "Dropout rate increased during movement"
            : verified
              ? "No movement-correlated failures remain"
              : "Movement correlation has not been tested",
        },
      ],
    },
    diagnosis: verified
      ? {
          headline: "Issue resolved",
          summary: "The ECHO signal is stable after the simulated repair and now matches its healthy baseline.",
          possible_causes: ["Previously intermittent ECHO connection"],
          confidence: 0.98,
          next_test: "Continue monitoring during normal operation.",
        }
      : wiggling
        ? {
            headline: "Movement correlation confirmed",
            summary: "ECHO failures repeatedly increased while the connection was moved. Power and TRIG remained stable.",
            possible_causes: ["Intermittent jumper wire", "Loose breadboard contact", "Poor ECHO pin connection"],
            confidence: 0.92,
            next_test: "Reseat or replace the ECHO jumper, then re-measure.",
          }
        : {
            headline: "Intermittent ECHO activity",
            summary: "The failure is isolated to the ECHO path. Current evidence does not yet prove a loose connection.",
            possible_causes: ["Intermittent connection", "Sensor malfunction", "Software-controlled switching"],
            confidence: 0.68,
            next_test: "Gently wiggle the ECHO jumper while P3 is monitored.",
          },
    before: { dropouts_per_minute: 12, stability: "intermittent" },
    after: verified ? { dropouts_per_minute: 0, stability: "stable" } : undefined,
    timeline: [
      { id: "detect", label: "Detect", detail: "Unexpected ECHO dropouts", time: "00:04", complete: true },
      { id: "diagnose", label: "Diagnose", detail: "Fault isolated to P3", time: "00:07", complete: true },
      { id: "test", label: "Test", detail: wiggling || verified ? "Movement correlation found" : "Wiggle test ready", time: "00:12", complete: wiggling || verified },
      { id: "verify", label: "Verify", detail: verified ? "Signal matches baseline" : "Waiting for repair", time: "00:18", complete: verified },
    ],
  };
}
