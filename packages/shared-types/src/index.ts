export type SessionStage = "detect" | "diagnose" | "test" | "repair" | "verify";

export type ProbeStatus = "stable" | "active" | "intermittent" | "idle";

export interface ProbeReading {
  probe: string;
  role: string;
  value: number | null;
  unit: string;
  status: ProbeStatus;
  dropouts: number;
  samples: number[];
}

export interface RuleResult {
  id: string;
  probe?: string;
  status: "pass" | "warn" | "fail";
  severity?: number;
  message: string;
  provenance?: EvidenceProvenance;
}

export type EvidenceProvenance =
  | "MEASURED"
  | "DERIVED"
  | "SPECIFICATION"
  | "BASELINE"
  | "SOFTWARE"
  | "AI_INTERPRETATION";

export interface EvidenceFact {
  probe?: string;
  name: string;
  value: unknown;
  unit?: string;
  provenance: EvidenceProvenance;
  detail?: string;
}

export interface StructuredEvidence {
  probe: string;
  role: string;
  expected: Record<string, string | number | boolean>;
  observed: Record<string, string | number | boolean>;
  baseline: Record<string, string | number | boolean>;
  measurements?: EvidenceFact[];
  derived_facts?: EvidenceFact[];
  specification_results?: EvidenceFact[];
  baseline_comparison?: EvidenceFact[];
  rule_results: RuleResult[];
  unresolved_questions?: string[];
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
  telemetry_mode?: "simulator" | "serial" | "browser" | string;
  profile_id?: string;
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

export interface ProjectProfile {
  id: string;
  project_name: string;
  controller: string;
  logic_voltage: number;
  confirmed: boolean;
  expected_behavior: string;
  components: Array<{
    id: string;
    name: string;
    manufacturer?: string;
    properties?: Record<string, number>;
    source?: string;
  }>;
  probes: Array<{
    probe: string;
    role: string;
    mode: "analog" | "digital" | "pulse";
    is_power_rail: boolean;
    expected: Record<string, unknown>;
    safe_measurement: {
      max_pin_voltage: number;
      input_scale: number;
      notes?: string;
    };
    baseline?: Record<string, unknown>;
  }>;
  created_at_ms: number;
  updated_at_ms: number;
}
