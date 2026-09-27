package domain

import "context"

type ProjectFactSource string

const (
	SourceCodeStaticAnalysis ProjectFactSource = "CODE_STATIC_ANALYSIS"
	SourceVisionAI           ProjectFactSource = "VISION_AI"
	SourceCatalog            ProjectFactSource = "CATALOG"
	SourceUser               ProjectFactSource = "USER"
	SourceInferred           ProjectFactSource = "INFERRED"
)

type AnalysisStatus string

const (
	AnalysisPending    AnalysisStatus = "PENDING"
	AnalysisProcessing AnalysisStatus = "PROCESSING"
	AnalysisDraftReady AnalysisStatus = "DRAFT_READY"
	AnalysisFailed     AnalysisStatus = "FAILED"
	AnalysisConfirmed  AnalysisStatus = "CONFIRMED"
)

type ProjectMedia struct {
	StorageRef       string `json:"storage_ref"`
	OriginalFilename string `json:"original_filename"`
	ContentType      string `json:"content_type"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256"`
}

type ProjectCode struct {
	Filename  string `json:"filename"`
	Language  string `json:"language"`
	Text      string `json:"text"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type CodePinFinding struct {
	GPIO       int               `json:"gpio"`
	Symbol     string            `json:"symbol"`
	Direction  string            `json:"direction"`
	Behavior   string            `json:"behavior"`
	Confidence float64           `json:"confidence"`
	Source     ProjectFactSource `json:"source"`
	Evidence   []string          `json:"evidence"`
}

type CodeAnalysis struct {
	Status    string           `json:"status"`
	Language  string           `json:"language"`
	Parser    string           `json:"parser"`
	Pins      []CodePinFinding `json:"pins"`
	Includes  []string         `json:"includes"`
	Libraries []string         `json:"libraries"`
	Timing    []string         `json:"timing"`
	Warnings  []string         `json:"warnings"`
}

type VisionComponent struct {
	CatalogID     string            `json:"catalog_id,omitempty"`
	Name          string            `json:"name"`
	Confidence    float64           `json:"confidence"`
	VisibleLabels []string          `json:"visible_labels,omitempty"`
	Source        ProjectFactSource `json:"source"`
}

type VisionRelationship struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Role       string            `json:"role"`
	GPIO       *int              `json:"gpio,omitempty"`
	Confidence float64           `json:"confidence"`
	Source     ProjectFactSource `json:"source"`
}

type VisionAnalysis struct {
	Status        string               `json:"status"`
	Model         string               `json:"model,omitempty"`
	Components    []VisionComponent    `json:"components"`
	Relationships []VisionRelationship `json:"relationships"`
	Warnings      []string             `json:"warnings"`
}

type ProjectAnalysis struct {
	Code          CodeAnalysis   `json:"code"`
	Vision        VisionAnalysis `json:"vision"`
	GeneratedAtMS int64          `json:"generated_at_ms"`
}

type ProbeInstruction struct {
	Probe       string `json:"probe"`
	Role        string `json:"role"`
	Target      string `json:"target"`
	Expected    string `json:"expected"`
	SignalType  string `json:"signal_type"`
	SafeWarning string `json:"safe_warning"`
	Explanation string `json:"explanation,omitempty"`
}

type ProbePlan struct {
	ProjectID     string             `json:"project_id"`
	ProfileID     string             `json:"profile_id"`
	Instructions  []ProbeInstruction `json:"instructions"`
	Connected     bool               `json:"connected"`
	ConnectedAtMS int64              `json:"connected_at_ms,omitempty"`
	GeneratedAtMS int64              `json:"generated_at_ms"`
}

type ProjectVisibility string

const (
	VisibilityPrivate ProjectVisibility = "private"
	VisibilityPublic  ProjectVisibility = "public"
)

type Project struct {
	ID string `json:"id"`
	// OwnerID is the verified Google account id that created this project,
	// or empty for anonymous/Demo Mode projects. Never trust a client-supplied
	// value for this — it is only ever set server-side from a verified token.
	OwnerID      string        `json:"owner_id,omitempty"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Controller   string        `json:"controller"`
	LogicVoltage float64       `json:"logic_voltage"`
	Image        *ProjectMedia `json:"image,omitempty"`
	Code         *ProjectCode  `json:"code,omitempty"`
	// Repository is the linked GitHub repo; its default branch supplies Code.
	Repository     *LinkedRepository `json:"repository,omitempty"`
	Analysis       *ProjectAnalysis  `json:"analysis,omitempty"`
	AnalysisStatus AnalysisStatus    `json:"analysis_status"`
	AnalysisError  string            `json:"analysis_error,omitempty"`
	ProbePlan      *ProbePlan        `json:"probe_plan,omitempty"`
	// Visibility is stored and displayed but not enforced yet: there is no
	// user auth, so every project is reachable by anyone who can reach the API.
	Visibility  ProjectVisibility `json:"visibility"`
	CreatedAtMS int64             `json:"created_at_ms"`
	UpdatedAtMS int64             `json:"updated_at_ms"`
}

type Stage string

const (
	StageDetect   Stage = "detect"
	StageDiagnose Stage = "diagnose"
	StageTest     Stage = "test"
	StageRepair   Stage = "repair"
	StageVerify   Stage = "verify"
)

type Provenance string

const (
	ProvenanceMeasured         Provenance = "MEASURED"
	ProvenanceDerived          Provenance = "DERIVED"
	ProvenanceSpecification    Provenance = "SPECIFICATION"
	ProvenanceBaseline         Provenance = "BASELINE"
	ProvenanceSoftware         Provenance = "SOFTWARE"
	ProvenanceAIInterpretation Provenance = "AI_INTERPRETATION"
)

type BaselineStatus string

const (
	BaselineUserConfirmedHealthy BaselineStatus = "USER_CONFIRMED_HEALTHY"
	BaselineKnownGoodCapture     BaselineStatus = "KNOWN_GOOD_CAPTURE"
	BaselineManufacturerSpec     BaselineStatus = "MANUFACTURER_SPEC"
	BaselineUnknown              BaselineStatus = "UNKNOWN"
)

func (status BaselineStatus) Trusted() bool {
	return status == BaselineUserConfirmedHealthy ||
		status == BaselineKnownGoodCapture ||
		status == BaselineManufacturerSpec
}

type ProbeMode string

const (
	ProbeModeAnalog  ProbeMode = "analog"
	ProbeModeDigital ProbeMode = "digital"
	ProbeModePulse   ProbeMode = "pulse"
)

type TelemetryEnvelope struct {
	Patch         *PatchCapability  `json:"patch,omitempty"`
	SchemaVersion int               `json:"schema_version"`
	DeviceID      string            `json:"device_id"`
	ProfileID     string            `json:"profile_id"`
	CapturedAtMS  int64             `json:"captured_at_ms"`
	UptimeMS      uint64            `json:"uptime_ms"`
	WindowMS      uint32            `json:"window_ms"`
	Sequence      uint64            `json:"sequence"`
	Samples       []TelemetrySample `json:"samples"`
}

// Optional Telemetry-v2 capability advertisement. An advertisement alone is
// never an arming handshake or permission to drive hardware.
type PatchCapability struct {
	Capable       bool   `json:"capable"`
	State         string `json:"state"`
	Reason        string `json:"reason"`
	BootID        uint32 `json:"boot_id"`
	MaxDurationMS uint32 `json:"max_duration_ms"`
}

type TelemetrySample struct {
	Probe             string    `json:"probe"`
	Mode              ProbeMode `json:"mode"`
	AnalogMV          []float64 `json:"analog_mv,omitempty"`
	State             *int      `json:"state,omitempty"`
	EdgeCount         uint32    `json:"edge_count,omitempty"`
	RisingEdges       uint32    `json:"rising_edges,omitempty"`
	FallingEdges      uint32    `json:"falling_edges,omitempty"`
	PeriodsUS         []float64 `json:"periods_us,omitempty"`
	HighPulseWidthsUS []float64 `json:"high_pulse_widths_us,omitempty"`
	MaxGapUS          uint64    `json:"max_gap_us,omitempty"`
	ActivityCounts    []float64 `json:"activity_counts,omitempty"`
}

type ExpectedSignal struct {
	SignalType          string   `json:"signal_type"`
	Required            bool     `json:"required"`
	Stable              bool     `json:"stable"`
	MinVoltage          *float64 `json:"min_voltage,omitempty"`
	MaxVoltage          *float64 `json:"max_voltage,omitempty"`
	NominalVoltage      *float64 `json:"nominal_voltage,omitempty"`
	VoltageTolerancePct float64  `json:"voltage_tolerance_pct,omitempty"`
	MinFrequencyHz      *float64 `json:"min_frequency_hz,omitempty"`
	MaxFrequencyHz      *float64 `json:"max_frequency_hz,omitempty"`
	NominalFrequencyHz  *float64 `json:"nominal_frequency_hz,omitempty"`
	MaxDropouts         int      `json:"max_dropouts"`
}

type SafeMeasurementConfig struct {
	MaxPinVoltage float64 `json:"max_pin_voltage"`
	InputScale    float64 `json:"input_scale"`
	Notes         string  `json:"notes,omitempty"`
}

type TrustedBaseline struct {
	Status                BaselineStatus `json:"status"`
	CapturedAtMS          int64          `json:"captured_at_ms,omitempty"`
	AverageVoltage        *float64       `json:"average_voltage,omitempty"`
	FrequencyHz           *float64       `json:"frequency_hz,omitempty"`
	DropoutsPerWindow     *int           `json:"dropouts_per_window,omitempty"`
	VoltageVariation      *float64       `json:"voltage_variation,omitempty"`
	FrequencyTolerancePct float64        `json:"frequency_tolerance_pct,omitempty"`
	VoltageTolerancePct   float64        `json:"voltage_tolerance_pct,omitempty"`
}

type ProbeConfiguration struct {
	Probe           string                `json:"probe"`
	Role            string                `json:"role"`
	Mode            ProbeMode             `json:"mode"`
	IsPowerRail     bool                  `json:"is_power_rail"`
	Expected        ExpectedSignal        `json:"expected"`
	SafeMeasurement SafeMeasurementConfig `json:"safe_measurement"`
	Baseline        *TrustedBaseline      `json:"baseline,omitempty"`
}

type ComponentSpecification struct {
	ID                   string              `json:"id"`
	Name                 string              `json:"name"`
	Manufacturer         string              `json:"manufacturer,omitempty"`
	Properties           map[string]float64  `json:"properties,omitempty"`
	Source               string              `json:"source,omitempty"`
	Sources              []ProjectFactSource `json:"sources,omitempty"`
	Confidence           float64             `json:"confidence,omitempty"`
	Confirmed            bool                `json:"confirmed"`
	InterfaceType        string              `json:"interface_type,omitempty"`
	ExpectedBehavior     string              `json:"expected_behavior,omitempty"`
	SafeMeasurementNotes []string            `json:"safe_measurement_notes,omitempty"`
}

type ProfileEvidence struct {
	Value      string            `json:"value"`
	Source     ProjectFactSource `json:"source"`
	Confidence float64           `json:"confidence"`
}

type ProfileConnection struct {
	ID            string              `json:"id"`
	ComponentID   string              `json:"component_id,omitempty"`
	ComponentName string              `json:"component_name"`
	Role          string              `json:"role"`
	GPIO          *int                `json:"gpio,omitempty"`
	Target        string              `json:"target"`
	Direction     string              `json:"direction"`
	Behavior      string              `json:"behavior"`
	Expected      ExpectedSignal      `json:"expected"`
	Confidence    float64             `json:"confidence"`
	Sources       []ProjectFactSource `json:"sources"`
	Evidence      []ProfileEvidence   `json:"evidence,omitempty"`
	Required      bool                `json:"required"`
	Confirmed     bool                `json:"confirmed"`
}

type ConflictOption struct {
	Value      string            `json:"value"`
	Source     ProjectFactSource `json:"source"`
	Confidence float64           `json:"confidence"`
}

type ProfileConflict struct {
	ID                   string           `json:"id"`
	ConnectionID         string           `json:"connection_id,omitempty"`
	Field                string           `json:"field"`
	Options              []ConflictOption `json:"options"`
	Resolution           string           `json:"resolution,omitempty"`
	Resolved             bool             `json:"resolved"`
	RequiresConfirmation bool             `json:"requires_confirmation"`
}

type ProjectProfile struct {
	ID                  string                   `json:"id"`
	ProjectID           string                   `json:"project_id,omitempty"`
	Version             int                      `json:"version"`
	ProjectName         string                   `json:"project_name"`
	Controller          string                   `json:"controller"`
	LogicVoltage        float64                  `json:"logic_voltage"`
	Confirmed           bool                     `json:"confirmed"`
	ConfirmedAtMS       int64                    `json:"confirmed_at_ms,omitempty"`
	ConfirmedBy         string                   `json:"confirmed_by,omitempty"`
	Components          []ComponentSpecification `json:"components"`
	Connections         []ProfileConnection      `json:"connections,omitempty"`
	Probes              []ProbeConfiguration     `json:"probes"`
	ExpectedBehavior    string                   `json:"expected_behavior"`
	OperatingConditions []string                 `json:"operating_conditions,omitempty"`
	Conflicts           []ProfileConflict        `json:"conflicts,omitempty"`
	UnresolvedQuestions []string                 `json:"unresolved_questions,omitempty"`
	AnalysisStatus      string                   `json:"analysis_status,omitempty"`
	CreatedAtMS         int64                    `json:"created_at_ms"`
	UpdatedAtMS         int64                    `json:"updated_at_ms"`
}

func (profile ProjectProfile) Probe(probe string) (ProbeConfiguration, bool) {
	for _, configuration := range profile.Probes {
		if configuration.Probe == probe {
			return configuration, true
		}
	}
	return ProbeConfiguration{}, false
}

type DerivedFacts struct {
	Probe                    string    `json:"probe"`
	Role                     string    `json:"role"`
	Mode                     ProbeMode `json:"mode"`
	AverageVoltage           *float64  `json:"average_voltage,omitempty"`
	MinimumVoltage           *float64  `json:"minimum_voltage,omitempty"`
	MaximumVoltage           *float64  `json:"maximum_voltage,omitempty"`
	VoltageVariation         *float64  `json:"voltage_variation,omitempty"`
	DigitalState             *int      `json:"digital_state,omitempty"`
	DigitalTransitions       uint32    `json:"digital_transitions"`
	PulseCount               uint32    `json:"pulse_count"`
	FrequencyHz              *float64  `json:"frequency_hz,omitempty"`
	DutyCyclePercent         *float64  `json:"duty_cycle_percent,omitempty"`
	AveragePulseWidthUS      *float64  `json:"average_pulse_width_us,omitempty"`
	MinimumPulseWidthUS      *float64  `json:"minimum_pulse_width_us,omitempty"`
	MaximumPulseWidthUS      *float64  `json:"maximum_pulse_width_us,omitempty"`
	JitterUS                 *float64  `json:"jitter_us,omitempty"`
	MaximumGapUS             *float64  `json:"maximum_gap_us,omitempty"`
	DropoutEvents            int       `json:"dropout_events"`
	MissingExpectedActivity  bool      `json:"missing_expected_activity"`
	Stable                   bool      `json:"stable"`
	RailStable               *bool     `json:"rail_stable,omitempty"`
	FailureBuckets           []int     `json:"failure_buckets,omitempty"`
	ActivityCounts           []float64 `json:"activity_counts,omitempty"`
	BaselineDeviationPercent *float64  `json:"baseline_deviation_percent,omitempty"`
}

type AnalysisResult struct {
	SchemaVersion             int            `json:"schema_version"`
	DeviceID                  string         `json:"device_id"`
	ProfileID                 string         `json:"profile_id"`
	CapturedAtMS              int64          `json:"captured_at_ms"`
	WindowMS                  uint32         `json:"window_ms"`
	Probes                    []DerivedFacts `json:"probes"`
	SimultaneousDropoutGroups [][]string     `json:"simultaneous_dropout_groups,omitempty"`
}

func (analysis AnalysisResult) Probe(probe string) (DerivedFacts, bool) {
	for _, facts := range analysis.Probes {
		if facts.Probe == probe {
			return facts, true
		}
	}
	return DerivedFacts{}, false
}

type EvidenceFact struct {
	Probe      string     `json:"probe,omitempty"`
	Name       string     `json:"name"`
	Value      any        `json:"value"`
	Unit       string     `json:"unit,omitempty"`
	Provenance Provenance `json:"provenance"`
	Detail     string     `json:"detail,omitempty"`
}

type RuleResult struct {
	ID         string     `json:"id"`
	Probe      string     `json:"probe,omitempty"`
	Status     string     `json:"status"`
	Severity   int        `json:"severity"`
	Message    string     `json:"message"`
	Provenance Provenance `json:"provenance"`
}

type Evidence struct {
	Probe                string         `json:"probe"`
	Role                 string         `json:"role"`
	Expected             map[string]any `json:"expected"`
	Observed             map[string]any `json:"observed"`
	Baseline             map[string]any `json:"baseline"`
	Measurements         []EvidenceFact `json:"measurements"`
	DerivedFacts         []EvidenceFact `json:"derived_facts"`
	SpecificationResults []EvidenceFact `json:"specification_results"`
	BaselineComparison   []EvidenceFact `json:"baseline_comparison"`
	RuleResults          []RuleResult   `json:"rule_results"`
	UnresolvedQuestions  []string       `json:"unresolved_questions"`
}

type Diagnosis struct {
	Headline       string   `json:"headline"`
	Summary        string   `json:"summary"`
	PossibleCauses []string `json:"possible_causes"`
	Confidence     float64  `json:"confidence"`
	NextTest       string   `json:"next_test"`
}

type ProbeReading struct {
	Probe    string    `json:"probe"`
	Role     string    `json:"role"`
	Value    *float64  `json:"value"`
	Unit     string    `json:"unit"`
	Status   string    `json:"status"`
	Dropouts int       `json:"dropouts"`
	Samples  []float64 `json:"samples"`
}

type MeasurementSummary struct {
	DropoutsPerMinute int    `json:"dropouts_per_minute"`
	Stability         string `json:"stability"`
}

type TimelineEvent struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	Time     string `json:"time"`
	Complete bool   `json:"complete"`
}

type VerifyResult string

const (
	VerifyFixed        VerifyResult = "fixed"
	VerifyStillFailing VerifyResult = "still_failing"
	VerifyUnclear      VerifyResult = "unclear"
)

type Report struct {
	SessionID      string              `json:"session_id"`
	ProjectName    string              `json:"project_name"`
	ProfileID      string              `json:"profile_id"`
	Stage          Stage               `json:"stage"`
	Headline       string              `json:"headline"`
	Summary        string              `json:"summary"`
	PossibleCauses []string            `json:"possible_causes"`
	Confidence     float64             `json:"confidence"`
	NextTest       string              `json:"next_test"`
	Before         MeasurementSummary  `json:"before"`
	After          *MeasurementSummary `json:"after,omitempty"`
	Verify         *VerifyResult       `json:"verify,omitempty"`
	Evidence       Evidence            `json:"evidence"`
}

type Session struct {
	ID                string              `json:"id"`
	ProjectName       string              `json:"project_name"`
	Stage             Stage               `json:"stage"`
	HardwareConnected bool                `json:"hardware_connected"`
	TelemetryMode     string              `json:"telemetry_mode"`
	ProfileID         string              `json:"profile_id"`
	ScenarioID        string              `json:"scenario_id,omitempty"`
	MeasurementID     int64               `json:"measurement_id,omitempty"`
	RawTelemetry      TelemetryEnvelope   `json:"raw_telemetry"`
	Probes            []ProbeReading      `json:"probes"`
	Analysis          AnalysisResult      `json:"analysis"`
	Evidence          Evidence            `json:"evidence"`
	Diagnosis         Diagnosis           `json:"diagnosis"`
	Before            MeasurementSummary  `json:"before"`
	After             *MeasurementSummary `json:"after,omitempty"`
	Timeline          []TimelineEvent     `json:"timeline"`
}

type TelemetrySource interface {
	Name() string
	Latest(ctx context.Context) (TelemetryEnvelope, error)
}

type ScenarioTelemetrySource interface {
	TelemetrySource
	SetStage(stage Stage)
}

type SimulatorScenario struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	ExpectedFinding string `json:"expected_finding"`
}

type FaultScenarioSource interface {
	ScenarioTelemetrySource
	Scenarios() []SimulatorScenario
	SetScenario(id string) error
	CurrentScenario() string
}

type FreshTelemetrySource interface {
	TelemetrySource
	WaitNext(ctx context.Context, afterSequence uint64) (TelemetryEnvelope, error)
}

type Repository interface {
	SaveSession(session Session) error
	LatestSession() (*Session, error)
	SaveProfile(profile ProjectProfile) error
	SaveProjectProfile(project Project, profile ProjectProfile) error
	GetProfile(id string) (*ProjectProfile, error)
	ListProfiles() ([]ProjectProfile, error)
	SaveProject(project Project) error
	// SetProjectVisibility changes only visibility; it is not a content
	// update, so it leaves updated_at alone.
	SetProjectVisibility(id string, visibility ProjectVisibility) error
	GetProject(id string) (*Project, error)
	ListProjects() ([]Project, error)
}

type MeasurementWindow struct {
	ID           int64             `json:"id"`
	ProfileID    string            `json:"profile_id"`
	Source       string            `json:"source"`
	DeviceID     string            `json:"device_id"`
	Sequence     uint64            `json:"sequence"`
	CapturedAtMS int64             `json:"captured_at_ms"`
	IngestedAtMS int64             `json:"ingested_at_ms"`
	Raw          TelemetryEnvelope `json:"raw"`
	Analysis     AnalysisResult    `json:"analysis"`
}

// MeasurementQuery scopes a QueryMeasurements read. Probe is matched
// against each window's samples (measurement_windows stores whole windows,
// not one row per probe, so this is a best-effort, non-indexed filter
// layered on top of the indexed profile_id/device_id/source/time-range
// filters) -- see SQLiteStore.QueryMeasurements for the exact semantics.
// Limit is always clamped by the implementation; it can never be
// unbounded.
type MeasurementQuery struct {
	ProfileID string
	DeviceID  string
	Source    string
	Probe     string
	SinceMS   *int64
	UntilMS   *int64
	Limit     int
}

type MeasurementRepository interface {
	SaveMeasurement(window MeasurementWindow) (MeasurementWindow, error)
	ListMeasurements(profileID string, limit int) ([]MeasurementWindow, error)
	QueryMeasurements(query MeasurementQuery) ([]MeasurementWindow, error)
}
