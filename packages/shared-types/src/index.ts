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
  scenario_id?: string;
  measurement_id?: number;
  raw_telemetry?: TelemetryEnvelope;
  analysis?: SignalAnalysis;
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

export interface TelemetrySample {
  probe: string;
  mode: "analog" | "digital" | "pulse";
  analog_mv?: number[];
  state?: 0 | 1;
  edge_count?: number;
  rising_edges?: number;
  falling_edges?: number;
  periods_us?: number[];
  high_pulse_widths_us?: number[];
  max_gap_us?: number;
  activity_counts?: number[];
}

export interface TelemetryEnvelope {
  schema_version: 2;
  device_id: string;
  profile_id: string;
  captured_at_ms: number;
  uptime_ms: number;
  window_ms: number;
  sequence: number;
  samples: TelemetrySample[];
}

export interface DerivedSignalFacts {
  probe: string;
  role: string;
  mode: "analog" | "digital" | "pulse";
  average_voltage?: number;
  minimum_voltage?: number;
  maximum_voltage?: number;
  voltage_variation?: number;
  digital_state?: number;
  digital_transitions: number;
  pulse_count: number;
  frequency_hz?: number;
  duty_cycle_percent?: number;
  average_pulse_width_us?: number;
  minimum_pulse_width_us?: number;
  maximum_pulse_width_us?: number;
  jitter_us?: number;
  maximum_gap_us?: number;
  dropout_events: number;
  missing_expected_activity: boolean;
  stable: boolean;
  rail_stable?: boolean;
  failure_buckets?: number[];
  activity_counts?: number[];
  baseline_deviation_percent?: number;
}

export interface SignalAnalysis {
  schema_version: number;
  device_id: string;
  profile_id: string;
  captured_at_ms: number;
  window_ms: number;
  probes: DerivedSignalFacts[];
  simultaneous_dropout_groups?: string[][];
}

export interface SimulatorScenario {
  id: string;
  name: string;
  description: string;
  expected_finding: string;
}

export interface SimulatorScenarioList {
  active: string;
  scenarios: SimulatorScenario[];
}

export interface MeasurementWindow {
  id: number;
  profile_id: string;
  source: string;
  device_id: string;
  sequence: number;
  captured_at_ms: number;
  ingested_at_ms: number;
  raw: TelemetryEnvelope;
  analysis: SignalAnalysis;
}

export interface ProjectProfile {
  id: string;
  project_id?: string;
  version: number;
  project_name: string;
  controller: string;
  logic_voltage: number;
  confirmed: boolean;
  confirmed_at_ms?: number;
  confirmed_by?: string;
  expected_behavior: string;
  components: ProfileComponent[];
  connections?: ProfileConnection[];
  conflicts?: ProfileConflict[];
  unresolved_questions?: string[];
  operating_conditions?: string[];
  analysis_status?: string;
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

export type ProjectFactSource =
  | "CODE_STATIC_ANALYSIS"
  | "VISION_AI"
  | "CATALOG"
  | "USER"
  | "INFERRED";

export interface ProfileComponent {
  id: string;
  name: string;
  manufacturer?: string;
  properties?: Record<string, number>;
  source?: string;
  sources?: ProjectFactSource[];
  confidence?: number;
  confirmed: boolean;
  interface_type?: string;
  expected_behavior?: string;
  safe_measurement_notes?: string[];
}

export interface ExpectedSignal {
  signal_type: string;
  required: boolean;
  stable: boolean;
  min_voltage?: number;
  max_voltage?: number;
  nominal_voltage?: number;
  voltage_tolerance_pct?: number;
  min_frequency_hz?: number;
  max_frequency_hz?: number;
  nominal_frequency_hz?: number;
  max_dropouts: number;
}

export interface ProfileEvidence {
  value: string;
  source: ProjectFactSource;
  confidence: number;
}

export interface ProfileConnection {
  id: string;
  component_id?: string;
  component_name: string;
  role: string;
  gpio?: number;
  target: string;
  direction: string;
  behavior: string;
  expected: ExpectedSignal;
  confidence: number;
  sources: ProjectFactSource[];
  evidence?: ProfileEvidence[];
  required: boolean;
  confirmed: boolean;
}

export interface ConflictOption {
  value: string;
  source: ProjectFactSource;
  confidence: number;
}

export interface ProfileConflict {
  id: string;
  connection_id?: string;
  field: string;
  options: ConflictOption[];
  resolution?: string;
  resolved: boolean;
  requires_confirmation: boolean;
}

export interface CodePinFinding {
  gpio: number;
  symbol: string;
  direction: string;
  behavior: string;
  confidence: number;
  source: ProjectFactSource;
  evidence: string[];
}

export interface CodeAnalysis {
  status: string;
  language: string;
  parser: string;
  pins: CodePinFinding[];
  includes: string[];
  libraries: string[];
  timing: string[];
  warnings: string[];
}

export interface VisionComponent {
  catalog_id?: string;
  name: string;
  confidence: number;
  visible_labels?: string[];
  source: "VISION_AI";
}

export interface VisionRelationship {
  from: string;
  to: string;
  role: string;
  gpio?: number;
  confidence: number;
  source: "VISION_AI";
}

export interface ProjectAnalysis {
  code: CodeAnalysis;
  vision: {
    status: string;
    model?: string;
    components: VisionComponent[];
    relationships: VisionRelationship[];
    warnings: string[];
  };
  generated_at_ms: number;
}

export interface ProjectMedia {
  storage_ref: string;
  original_filename: string;
  content_type: string;
  size_bytes: number;
  sha256: string;
}

export interface ProjectCode {
  filename: string;
  language: string;
  text: string;
  size_bytes: number;
  sha256: string;
}

export interface ProbeInstruction {
  probe: string;
  role: string;
  target: string;
  expected: string;
  signal_type: string;
  safe_warning: string;
  explanation?: string;
}

export interface ProbePlan {
  project_id: string;
  profile_id: string;
  instructions: ProbeInstruction[];
  connected: boolean;
  connected_at_ms?: number;
  generated_at_ms: number;
}

export interface Project {
  id: string;
  name: string;
  description?: string;
  controller: string;
  logic_voltage: number;
  image?: ProjectMedia;
  code?: ProjectCode;
  analysis?: ProjectAnalysis;
  analysis_status: "PENDING" | "PROCESSING" | "DRAFT_READY" | "FAILED" | "CONFIRMED";
  analysis_error?: string;
  probe_plan?: ProbePlan;
  created_at_ms: number;
  updated_at_ms: number;
}

export interface AnalyzeProjectResponse {
  project: Project;
  analysis: ProjectAnalysis;
  profile: ProjectProfile;
}

export interface ConfirmProfileResponse {
  profile: ProjectProfile;
  probe_plan: ProbePlan;
}
