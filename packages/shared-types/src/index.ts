export type SessionStage = "detect" | "diagnose" | "test" | "repair" | "verify";

export type ProbeStatus = "stable" | "active" | "intermittent" | "idle";

export interface ProbeReading {
  probe: string;
  role: string;
  value: number | null;
  unit: string;
  status: ProbeStatus;
  dropouts: number;
  samples: number[] | null;
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
  /** A captured HIGH time violated the configured pulse-width expectation. */
  pulse_width_out_of_range?: boolean;
  /** The raw capture is internally inconsistent; raw values are unchanged. */
  capture_unreliable?: boolean;
  capture_issues?: string[];
  /** Metrics outside a learned physical Known Good envelope. */
  known_good_deviations?: string[];
}

export interface SignalAnalysis {
  schema_version: number;
  device_id: string;
  profile_id: string;
  profile_version?: number;
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

export type BaselineSource = "PHYSICAL" | "SIMULATED";
export type PassportStatus = "NO_PHYSICAL_BASELINE" | "NEEDS_VERIFICATION" | "HEALTHY" | "DEVIATION_DETECTED" | "SIMULATED_BASELINE" | "SIMULATED_MATCH" | "SIMULATED_DEVIATION" | "BASELINE_INCOMPATIBLE";
export type MeasurementProvenance = "REAL_SERIAL" | "SIMULATED";

/** Recorded physical behavior. Never a configured expectation. */
export interface TrustedBaseline {
  status: "USER_CONFIRMED_HEALTHY" | "KNOWN_GOOD_CAPTURE" | "MANUFACTURER_SPEC" | "UNKNOWN";
  captured_at_ms?: number;
  average_voltage?: number;
  frequency_hz?: number;
  dropouts_per_window?: number;
  voltage_variation?: number;
  frequency_tolerance_pct?: number;
  voltage_tolerance_pct?: number;
  window_count?: number;
  min_voltage?: number;
  max_voltage?: number;
  min_frequency_hz?: number;
  max_frequency_hz?: number;
  pulse_width_us?: number;
  min_pulse_width_us?: number;
  max_pulse_width_us?: number;
  pulse_width_tolerance_pct?: number;
  compare_pulse_width?: boolean;
}

export interface ProbeMappingEntry {
  probe: string;
  role: string;
  mode: "analog" | "digital" | "pulse";
  input_scale: number;
  required: boolean;
}

export interface KnownGoodBaseline {
  id: number;
  profile_id: string;
  profile_version: number;
  measurement_id: number;
  source: BaselineSource;
  device_id: string;
  captured_at_ms: number;
  ingested_at_ms: number;
  saved_at_ms: number;
  confirmed_by: "local_user_reported";
  note?: string;
  probes: Array<{
    probe: string;
    role: string;
    facts: DerivedSignalFacts;
    trusted: TrustedBaseline;
  }>;
  provenance?: MeasurementProvenance;
  probe_mapping?: ProbeMappingEntry[];
  probe_mapping_hash?: string;
  window_count?: number;
  first_measurement_id?: number;
  last_measurement_id?: number;
}

export type CalibrationStatus =
  | "NOT_CALIBRATED"
  | "OBSERVING"
  | "ATTENTION_REQUIRED"
  | "REVIEW_REQUIRED"
  | "CALIBRATED"
  | "BASELINE_INCOMPATIBLE"
  | "SIMULATED_SOURCE";

export interface ObservedSummary {
  windows: number;
  stable_windows: number;
  unreliable_windows: number;
  max_dropouts: number;
  active_windows: number;
  min_voltage?: number;
  average_voltage?: number;
  max_voltage?: number;
  max_voltage_variation?: number;
  min_frequency_hz?: number;
  average_frequency_hz?: number;
  max_frequency_hz?: number;
  min_pulse_width_us?: number;
  average_pulse_width_us?: number;
  max_pulse_width_us?: number;
  capture_issues?: string[];
}

export interface CalibrationProbe {
  probe: string;
  role: string;
  mode: "analog" | "digital" | "pulse";
  required: boolean;
  expected: ExpectedSignal;
  observed: ObservedSummary;
  known_good?: TrustedBaseline;
  issues?: string[];
}

export interface CalibrationState {
  status: CalibrationStatus;
  detail: string;
  profile_id: string;
  profile_version: number;
  device_id?: string;
  provenance?: MeasurementProvenance;
  windows_observed: number;
  windows_required: number;
  candidate_measurement_id?: number;
  first_measurement_id?: number;
  can_save_known_good: boolean;
  blockers?: string[];
  probes: CalibrationProbe[];
  known_good?: KnownGoodBaseline;
  probe_mapping_hash: string;
}

export interface PassportCapture {
  measurement_id: number;
  source: BaselineSource;
  device_id: string;
  captured_at_ms: number;
  ingested_at_ms: number;
  window_ms: number;
  probes: DerivedSignalFacts[];
}

export interface DevicePassport {
  profile: ProjectProfile;
  project?: Project;
  probe_plan?: ProbePlan;
  known_good: KnownGoodBaseline[];
  physical_baseline?: KnownGoodBaseline;
  simulated_baseline?: KnownGoodBaseline;
  recent_captures: PassportCapture[];
  history: HistorySummary[];
  verified_repairs: Array<{ workflow_id: string; source: BaselineSource; device_id: string; verified_at_ms: number; summary: string; user_reported_actions: string[] }>;
  status: PassportStatus;
  status_detail: string;
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
  reserved_probes?: string[];
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
  | "INFERRED"
  | "REAL_SERIAL_OBSERVATION"
  | "HARDWARE_CONTRACT";

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
  min_pulse_width_us?: number;
  max_pulse_width_us?: number;
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
  /** Pins this connection to a physical ReWeird probe (P1-P6). */
  probe?: string;
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

export interface VisionAnalysis {
  status: string;
  model?: string;
  components: VisionComponent[];
  relationships: VisionRelationship[];
  warnings: string[];
}

export interface ProjectAnalysis {
  code: CodeAnalysis;
  vision: VisionAnalysis;
  generated_at_ms: number;
}

// PhysicalCommitVisionAnalysis is a persisted AI interpretation of a
// Physical Commit's raw image -- never the raw image itself, and never
// merged into the ProjectProfile/Circuit Map. Only ever exists when Gemini
// genuinely succeeded (status "VISION_COMPLETE"); a skipped or failed
// attempt is never stored.
export interface PhysicalCommitVisionAnalysis {
  id: string;
  project_id: string;
  physical_commit_id: string;
  provider: string;
  analysis: VisionAnalysis;
  created_at_ms: number;
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
  /** The verified Google account id that created this project; empty for anonymous/Demo Mode projects. */
  owner_id?: string;
  name: string;
  description?: string;
  controller: string;
  logic_voltage: number;
  image?: ProjectMedia;
  code?: ProjectCode;
  /** The GitHub repo the project's code comes from; pushes to its default branch are analyzed. */
  repository?: LinkedRepository;
  analysis?: ProjectAnalysis;
  analysis_status: "PENDING" | "PROCESSING" | "DRAFT_READY" | "FAILED" | "CONFIRMED";
  analysis_error?: string;
  probe_plan?: ProbePlan;
  /** Stored and shown, not enforced: there is no user auth yet. */
  visibility: "private" | "public";
  created_at_ms: number;
  updated_at_ms: number;
}

export type RepositorySyncStatus = "PENDING" | "SYNCING" | "SYNCED" | "FAILED" | "BLOCKED";

export interface RepositoryCommit {
  sha: string;
  message: string;
  author_name: string;
  committed_at_ms: number;
  html_url: string;
}

export interface LinkedRepository {
  id: number;
  full_name: string;
  default_branch: string;
  html_url: string;
  private: boolean;
  installation_id: number;
  /** The commit the current code analysis came from. */
  last_commit?: RepositoryCommit;
  latest_seen_sha?: string;
  sync_status: RepositorySyncStatus;
  sync_error?: string;
  synced_at_ms?: number;
  analyzed_files?: string[];
  skipped_files?: number;
}

export interface GitHubStatus {
  /** False when the server has no GitHub App settings. */
  configured: boolean;
  connected: boolean;
  install_url?: string;
  /** GitHub's authorize step; finds an existing installation without the install page. */
  authorize_url?: string;
  /** The installation's account (a user or an organization). */
  account_login?: string;
  /** The GitHub user who approved the connection. */
  github_user?: string;
  account_type?: string;
  connected_at_ms?: number;
}

export interface GitHubRepo {
  id: number;
  name: string;
  full_name: string;
  description: string;
  private: boolean;
  default_branch: string;
  html_url: string;
  language: string;
  pushed_at_ms: number;
  archived: boolean;
}

export interface GitHubRepoList {
  account_login: string;
  items: GitHubRepo[];
}

/** Returned by a repo sync; profile and analysis are present when a new commit was analyzed. */
export interface RepositorySyncResponse {
  project: Project;
  analysis?: ProjectAnalysis;
  profile?: ProjectProfile;
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

export type TestType = "MOVEMENT_CORRELATION" | "POWER_RAIL_STABILITY" | "SIMULTANEOUS_DROPOUT" | "SIGNAL_ACTIVITY" | "BASELINE_COMPARISON" | "FREQUENCY_TIMING" | "REMEASURE";
export type TestState = "PLANNED" | "READY" | "CAPTURING_BASELINE" | "WAITING_FOR_USER" | "CAPTURING_TEST" | "ANALYZING" | "COMPLETED" | "INCONCLUSIVE" | "CANCELLED" | "FAILED" | "LOCKED" | "VERIFYING" | "RESOLVED" | "UNRESOLVED";

export interface TestRecommendation {
  id: string;
  session_id: string;
  test_type: TestType;
  target_probes: string[];
  reason: string;
  instructions?: string[];
  duration_seconds: number;
  requires_user_action: boolean;
  requires_patch: boolean;
  status?: string;
}

export interface TestPlan {
  id: string;
  recommendation: TestRecommendation;
  title: string;
  instructions: string[];
  monitoring: string[];
  metrics: string[];
  criteria: string;
  window_ms: number;
  requires_patch: boolean;
  unavailable?: string;
}

export interface TestObservation {
  probe?: string;
  metric: string;
  value: unknown;
  unit?: string;
  provenance: EvidenceProvenance | "GUIDED_TEST";
}

export interface TestResult {
  test_id: string;
  test_type: TestType;
  target_probes: string[];
  observations: TestObservation[];
  derived_metrics: Record<string, unknown>;
  result: string;
  interpretation: string;
  confidence: number;
  evidence_provenance: Array<EvidenceProvenance | "GUIDED_TEST">;
  timestamp_ms: number;
}

export interface MetricChange {
  probe: string;
  metric: string;
  before: unknown;
  after: unknown;
  unit?: string;
}

export interface VerificationResult {
  status: "RESOLVED" | "IMPROVED" | "UNCHANGED" | "WORSE" | "INCONCLUSIVE";
  improvements: MetricChange[];
  remaining_issues: string[];
  changes: MetricChange[];
  summary: string;
  before_window_id: number;
  after_window_id: number;
  timestamp_ms: number;
}

export interface DiagnosticWorkflow {
  id: string;
  session_id: string;
  project_id: string;
  profile_id: string;
  profile_version: number;
  profile_snapshot?: ProjectProfile;
  scenario_id?: string;
  status: TestState;
  plan: TestPlan;
  baseline?: MeasurementWindow;
  during?: MeasurementWindow;
  after?: MeasurementWindow;
  result?: TestResult;
  verification?: VerificationResult;
  user_actions?: UserAction[];
  error?: string;
  created_at_ms: number;
  updated_at_ms: number;
}

export interface UserAction { id: string; description: string; timestamp_ms: number }

// PhysicalCommit is a point-in-time snapshot of a real project's physical
// state (Circuit Map/component state, referenced Device Passport baseline
// and measurement, optional photo). Every evidence field is independently
// optional -- a normal project with none of this evidence yet still
// produces a valid commit. Software fields are reserved for a later GitHub
// integration milestone and stay undefined for now.
export interface PhysicalCommit {
  id: string;
  project_id: string;
  sequence: number;
  display_id: string;
  note?: string;
  created_at_ms: number;
  image?: ProjectMedia;
  profile_snapshot?: ProjectProfile;
  passport_baseline_ids?: number[];
  measurement_id?: number;
  software_provider?: string;
  software_repository?: string;
  software_revision?: string;
}

// PhysicalCommitDetail is the read-time-enriched view from
// GET .../physical-commits/:commitId/detail. It resolves the commit's
// referenced measurement for display; the stored commit itself is
// unchanged, and a resolution failure simply omits `measurement` rather
// than fabricating one.
export interface PhysicalCommitDetail {
  commit: PhysicalCommit;
  measurement?: MeasurementWindow;
}

// Six distinct outcomes -- "not captured" and "unavailable" must never
// collapse into "unchanged".
export type EvidenceState = "UNCHANGED" | "CHANGED" | "ADDED" | "REMOVED" | "NOT_CAPTURED" | "UNAVAILABLE";

export interface FieldChange {
  field: string;
  before?: unknown;
  after?: unknown;
}
export interface ComponentChange { component_id: string; name?: string; status: EvidenceState; fields?: FieldChange[] }
export interface ConnectionChange { connection_id: string; summary?: string; status: EvidenceState; fields?: FieldChange[] }
export interface ProbeElectricalChange { probe: string; status: EvidenceState; fields?: FieldChange[] }
export interface VisualDiff { status: EvidenceState; before_image?: ProjectMedia; after_image?: ProjectMedia }
export interface ComponentsDiff { status: EvidenceState; changes?: ComponentChange[] }
export interface CircuitDiff { status: EvidenceState; changes?: ConnectionChange[] }
export interface ElectricalDiff { status: EvidenceState; before_measurement_id?: number; after_measurement_id?: number; probes?: ProbeElectricalChange[] }
export interface SoftwareDiff { status: EvidenceState; fields?: FieldChange[] }

// SemanticVisionComponentChange is a delta between two stored Gemini Vision
// interpretations, grouped by component identity (catalog_id when
// available, else normalized name) and compared by COUNT only -- never a
// claim about a specific physical instance.
export interface SemanticVisionComponentChange {
  key: string;
  name: string;
  status: EvidenceState;
  before_count: number;
  after_count: number;
}

// SemanticVisualDiff compares two commits' already-persisted AI
// interpretations (VisionAnalysis), not the raw images themselves -- see
// `visual` for the raw image evidence comparison. Computing this never
// triggers a new Gemini call; it is a pure comparison of what was already
// stored the last time each commit was explicitly analyzed.
export interface SemanticVisualDiff { status: EvidenceState; from_analyzed: boolean; to_analyzed: boolean; changes?: SemanticVisionComponentChange[] }

// PhysicalCommitDiff is a deterministic, structured comparison of two
// commits in the same project -- computed by the backend, never generated
// as prose. The frontend renders fixed labels off `status`, it does not
// draw its own conclusions from raw fields.
export interface PhysicalCommitDiff {
  project_id: string;
  from_commit: string;
  to_commit: string;
  visual: VisualDiff;
  components: ComponentsDiff;
  circuit: CircuitDiff;
  electrical: ElectricalDiff;
  software: SoftwareDiff;
  semantic_visual: SemanticVisualDiff;
}

export type HistoryStatus ="OPEN" | "TESTING" | "WAITING_FOR_USER" | "VERIFYING" | "RESOLVED" | "IMPROVED" | "UNRESOLVED" | "CANCELLED" | "INCONCLUSIVE";
export interface HistorySummary {
  id: string;
  project_id: string;
  project_name: string;
  session_id: string;
  profile_id: string;
  profile_version: number;
  telemetry_source?: string;
  original_problem: string;
  status: HistoryStatus;
  started_at_ms: number;
  ended_at_ms?: number;
}
export interface HistoryEvent { id: string; timestamp_ms: number; kind: string; description: string; provenance: EvidenceProvenance | "GUIDED_TEST" | "USER"; window_id?: number }
export interface HistoryDetail { summary: HistorySummary; timeline: HistoryEvent[]; workflow: DiagnosticWorkflow }
export interface ReportFact { probe?: string; window_id?: number; metric: string; value: unknown; unit?: string; provenance: string }
export interface DetailedReport {
  report_id: string; project_id: string; project_name: string; session_id: string; date_ms: number; controller?: string;
  profile_id: string; profile_revision: number; summary: string; observed_behavior: string; expected_behavior?: string;
  measured_evidence: ReportFact[]; derived_evidence: ReportFact[]; tests_performed: TestPlan[]; test_results: TestResult[];
  user_actions: UserAction[]; before_window_id?: number; after_window_id?: number; verify_result?: VerificationResult;
  final_status: HistoryStatus; unresolved_items: string[]; provenance: string[]; system_information: Record<string, unknown>;
  security_redaction_count: number;
}
