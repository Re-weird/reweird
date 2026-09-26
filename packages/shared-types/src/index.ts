export type SessionStage = "detect" | "diagnose" | "test" | "repair" | "verify";

export type ProbeStatus = "stable" | "active" | "intermittent" | "idle";

export interface ProbeReading {
  probe: string;
  role: "POWER" | "TRIG" | "ECHO" | "UNASSIGNED";
  value: number | null;
  unit: "V" | "kHz" | "pulses/s" | "";
  status: ProbeStatus;
  dropouts: number;
  samples: number[];
}

export interface RuleResult {
  id: string;
  status: "pass" | "warn" | "fail";
  message: string;
}

export interface StructuredEvidence {
  probe: string;
  role: string;
  expected: Record<string, string | number | boolean>;
  observed: Record<string, string | number | boolean>;
  baseline: Record<string, string | number | boolean>;
  rule_results: RuleResult[];
}

export interface TimelineEvent {
  id: string;
  label: string;
  detail: string;
  time: string;
  complete: boolean;
}

export interface DemoSession {
  id: string;
  project_name: string;
  stage: SessionStage;
  hardware_connected: boolean;
  probes: ProbeReading[];
  evidence: StructuredEvidence;
  diagnosis: {
    headline: string;
    summary: string;
    possible_causes: string[];
    confidence: number;
    next_test: string;
  };
  before: { dropouts_per_minute: number; stability: string };
  after?: { dropouts_per_minute: number; stability: string };
  timeline: TimelineEvent[];
}
