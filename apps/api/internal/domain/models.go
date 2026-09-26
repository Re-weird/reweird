package domain

import "context"

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
	SchemaVersion int               `json:"schema_version"`
	DeviceID      string            `json:"device_id"`
	CapturedAtMS  int64             `json:"captured_at_ms"`
	UptimeMS      uint64            `json:"uptime_ms"`
	WindowMS      uint32            `json:"window_ms"`
	Sequence      uint64            `json:"sequence"`
	Samples       []TelemetrySample `json:"samples"`
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
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Manufacturer string             `json:"manufacturer,omitempty"`
	Properties   map[string]float64 `json:"properties,omitempty"`
	Source       string             `json:"source,omitempty"`
}

type ProjectProfile struct {
	ID               string                   `json:"id"`
	ProjectName      string                   `json:"project_name"`
	Controller       string                   `json:"controller"`
	LogicVoltage     float64                  `json:"logic_voltage"`
	Confirmed        bool                     `json:"confirmed"`
	Components       []ComponentSpecification `json:"components"`
	Probes           []ProbeConfiguration     `json:"probes"`
	ExpectedBehavior string                   `json:"expected_behavior"`
	CreatedAtMS      int64                    `json:"created_at_ms"`
	UpdatedAtMS      int64                    `json:"updated_at_ms"`
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
	JitterUS                 *float64  `json:"jitter_us,omitempty"`
	DropoutEvents            int       `json:"dropout_events"`
	MissingExpectedActivity  bool      `json:"missing_expected_activity"`
	Stable                   bool      `json:"stable"`
	FailureBuckets           []int     `json:"failure_buckets,omitempty"`
	ActivityCounts           []float64 `json:"activity_counts,omitempty"`
	BaselineDeviationPercent *float64  `json:"baseline_deviation_percent,omitempty"`
}

type AnalysisResult struct {
	SchemaVersion             int            `json:"schema_version"`
	DeviceID                  string         `json:"device_id"`
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

type Session struct {
	ID                string              `json:"id"`
	ProjectName       string              `json:"project_name"`
	Stage             Stage               `json:"stage"`
	HardwareConnected bool                `json:"hardware_connected"`
	TelemetryMode     string              `json:"telemetry_mode"`
	ProfileID         string              `json:"profile_id"`
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

type FreshTelemetrySource interface {
	TelemetrySource
	WaitNext(ctx context.Context, afterSequence uint64) (TelemetryEnvelope, error)
}

type Repository interface {
	SaveSession(session Session) error
	LatestSession() (*Session, error)
	SaveProfile(profile ProjectProfile) error
	GetProfile(id string) (*ProjectProfile, error)
	ListProfiles() ([]ProjectProfile, error)
}
