package domain

// These contracts are the boundary between a recommendation provider (the
// deterministic engine today, PROBE later) and guided measurement workflows.
type TestType string

const (
	TestMovementCorrelation TestType = "MOVEMENT_CORRELATION"
	TestPowerRailStability  TestType = "POWER_RAIL_STABILITY"
	TestSimultaneousDropout TestType = "SIMULTANEOUS_DROPOUT"
	TestSignalActivity      TestType = "SIGNAL_ACTIVITY"
	TestBaselineComparison  TestType = "BASELINE_COMPARISON"
	TestFrequencyTiming     TestType = "FREQUENCY_TIMING"
	TestRemeasure           TestType = "REMEASURE"
)

type TestState string

const (
	TestPlanned           TestState = "PLANNED"
	TestReady             TestState = "READY"
	TestCapturingBaseline TestState = "CAPTURING_BASELINE"
	TestWaitingForUser    TestState = "WAITING_FOR_USER"
	TestCapturingTest     TestState = "CAPTURING_TEST"
	TestAnalyzing         TestState = "ANALYZING"
	TestCompleted         TestState = "COMPLETED"
	TestInconclusive      TestState = "INCONCLUSIVE"
	TestCancelled         TestState = "CANCELLED"
	TestFailed            TestState = "FAILED"
	TestLocked            TestState = "LOCKED"
	TestVerifying         TestState = "VERIFYING"
	TestResolved          TestState = "RESOLVED"
	TestUnresolved        TestState = "UNRESOLVED"
)

const ProvenanceGuidedTest Provenance = "GUIDED_TEST"

type TestRecommendation struct {
	ID                 string   `json:"id"`
	SessionID          string   `json:"session_id"`
	TestType           TestType `json:"test_type"`
	TargetProbes       []string `json:"target_probes"`
	Reason             string   `json:"reason"`
	Instructions       []string `json:"instructions,omitempty"`
	DurationSeconds    int      `json:"duration_seconds"`
	RequiresUserAction bool     `json:"requires_user_action"`
	RequiresPatch      bool     `json:"requires_patch"`
	Status             string   `json:"status,omitempty"`
}

type TestPlan struct {
	ID             string             `json:"id"`
	Recommendation TestRecommendation `json:"recommendation"`
	Title          string             `json:"title"`
	Instructions   []string           `json:"instructions"`
	Monitoring     []string           `json:"monitoring"`
	Metrics        []string           `json:"metrics"`
	Criteria       string             `json:"criteria"`
	WindowMS       uint32             `json:"window_ms"`
	RequiresPatch  bool               `json:"requires_patch"`
	Unavailable    string             `json:"unavailable,omitempty"`
}

type TestObservation struct {
	Probe      string     `json:"probe,omitempty"`
	Metric     string     `json:"metric"`
	Value      any        `json:"value"`
	Unit       string     `json:"unit,omitempty"`
	Provenance Provenance `json:"provenance"`
}

// TestResult is the stable output consumed by future Layer 5 reasoning.
type TestResult struct {
	TestID             string            `json:"test_id"`
	TestType           TestType          `json:"test_type"`
	TargetProbes       []string          `json:"target_probes"`
	Observations       []TestObservation `json:"observations"`
	DerivedMetrics     map[string]any    `json:"derived_metrics"`
	Result             string            `json:"result"`
	Interpretation     string            `json:"interpretation"`
	Confidence         float64           `json:"confidence"`
	EvidenceProvenance []Provenance      `json:"evidence_provenance"`
	TimestampMS        int64             `json:"timestamp_ms"`
}

type MetricChange struct {
	Probe  string `json:"probe"`
	Metric string `json:"metric"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Unit   string `json:"unit,omitempty"`
}

type VerificationResult struct {
	Status          string         `json:"status"`
	Improvements    []MetricChange `json:"improvements"`
	RemainingIssues []string       `json:"remaining_issues"`
	Changes         []MetricChange `json:"changes"`
	Summary         string         `json:"summary"`
	BeforeWindowID  int64          `json:"before_window_id"`
	AfterWindowID   int64          `json:"after_window_id"`
	TimestampMS     int64          `json:"timestamp_ms"`
}

// DiagnosticWorkflow persists the entire guided test across API restarts.
type DiagnosticWorkflow struct {
	ID              string              `json:"id"`
	SessionID       string              `json:"session_id"`
	ProjectID       string              `json:"project_id"`
	ProfileID       string              `json:"profile_id"`
	ProfileVersion  int                 `json:"profile_version"`
	ProfileSnapshot *ProjectProfile     `json:"profile_snapshot,omitempty"`
	ScenarioID      string              `json:"scenario_id,omitempty"`
	Status          TestState           `json:"status"`
	Plan            TestPlan            `json:"plan"`
	Baseline        *MeasurementWindow  `json:"baseline,omitempty"`
	During          *MeasurementWindow  `json:"during,omitempty"`
	After           *MeasurementWindow  `json:"after,omitempty"`
	Result          *TestResult         `json:"result,omitempty"`
	Verification    *VerificationResult `json:"verification,omitempty"`
	UserActions     []UserAction        `json:"user_actions,omitempty"`
	Error           string              `json:"error,omitempty"`
	CreatedAtMS     int64               `json:"created_at_ms"`
	UpdatedAtMS     int64               `json:"updated_at_ms"`
}

type UserAction struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	TimestampMS int64  `json:"timestamp_ms"`
}

type TestWorkflowRepository interface {
	SaveTestWorkflow(workflow DiagnosticWorkflow) error
	GetTestWorkflow(id string) (*DiagnosticWorkflow, error)
	LatestTestWorkflow(profileID string) (*DiagnosticWorkflow, error)
}
