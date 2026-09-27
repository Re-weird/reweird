package domain

import "errors"

var ErrKnownGoodExists = errors.New("this measurement is already saved as known good")

type BaselineSource string

const (
	BaselinePhysical  BaselineSource = "PHYSICAL"
	BaselineSimulated BaselineSource = "SIMULATED"
)

// KnownGoodBaseline is an immutable user-marked reference to one stored capture.
// Source and device identity come from the server-side measurement record.
type KnownGoodBaseline struct {
	ID             int64           `json:"id"`
	ProfileID      string          `json:"profile_id"`
	ProfileVersion int             `json:"profile_version"`
	MeasurementID  int64           `json:"measurement_id"`
	Source         BaselineSource  `json:"source"`
	DeviceID       string          `json:"device_id"`
	CapturedAtMS   int64           `json:"captured_at_ms"`
	IngestedAtMS   int64           `json:"ingested_at_ms"`
	SavedAtMS      int64           `json:"saved_at_ms"`
	ConfirmedBy    string          `json:"confirmed_by"`
	Note           string          `json:"note,omitempty"`
	Probes         []BaselineProbe `json:"probes"`
	// Provenance names the measurement path, e.g. REAL_SERIAL. It is derived
	// from the stored capture's source on the server, never from the client.
	Provenance MeasurementProvenance `json:"provenance,omitempty"`
	// ProbeMapping is the confirmed probe configuration the baseline was
	// learned under; ProbeMappingHash lets later captures detect a remap.
	ProbeMapping     []ProbeMappingEntry `json:"probe_mapping,omitempty"`
	ProbeMappingHash string              `json:"probe_mapping_hash,omitempty"`
	// Learned baselines cover a run of consecutive windows.
	WindowCount        int   `json:"window_count,omitempty"`
	FirstMeasurementID int64 `json:"first_measurement_id,omitempty"`
	LastMeasurementID  int64 `json:"last_measurement_id,omitempty"`
}

type MeasurementProvenance string

const (
	ProvenanceRealSerial     MeasurementProvenance = "REAL_SERIAL"
	ProvenanceSimulatedInput MeasurementProvenance = "SIMULATED"
)

type ProbeMappingEntry struct {
	Probe      string    `json:"probe"`
	Role       string    `json:"role"`
	Mode       ProbeMode `json:"mode"`
	InputScale float64   `json:"input_scale"`
	Required   bool      `json:"required"`
}

type BaselineProbe struct {
	Probe   string          `json:"probe"`
	Role    string          `json:"role"`
	Facts   DerivedFacts    `json:"facts"`
	Trusted TrustedBaseline `json:"trusted"`
}

type PassportCapture struct {
	MeasurementID int64          `json:"measurement_id"`
	Source        BaselineSource `json:"source"`
	DeviceID      string         `json:"device_id"`
	CapturedAtMS  int64          `json:"captured_at_ms"`
	IngestedAtMS  int64          `json:"ingested_at_ms"`
	WindowMS      uint32         `json:"window_ms"`
	Probes        []DerivedFacts `json:"probes"`
}

type PassportRepair struct {
	WorkflowID          string         `json:"workflow_id"`
	Source              BaselineSource `json:"source"`
	DeviceID            string         `json:"device_id"`
	VerifiedAtMS        int64          `json:"verified_at_ms"`
	Summary             string         `json:"summary"`
	UserReportedActions []string       `json:"user_reported_actions"`
}

type PassportStatus string

const (
	PassportNoPhysicalBaseline   PassportStatus = "NO_PHYSICAL_BASELINE"
	PassportNeedsVerification    PassportStatus = "NEEDS_VERIFICATION"
	PassportHealthy              PassportStatus = "HEALTHY"
	PassportDeviation            PassportStatus = "DEVIATION_DETECTED"
	PassportSimulatedBaseline    PassportStatus = "SIMULATED_BASELINE"
	PassportSimulatedMatch       PassportStatus = "SIMULATED_MATCH"
	PassportSimulatedDeviation   PassportStatus = "SIMULATED_DEVIATION"
	PassportBaselineIncompatible PassportStatus = "BASELINE_INCOMPATIBLE"
)

type DevicePassport struct {
	Profile           ProjectProfile      `json:"profile"`
	Project           *Project            `json:"project,omitempty"`
	ProbePlan         *ProbePlan          `json:"probe_plan,omitempty"`
	KnownGood         []KnownGoodBaseline `json:"known_good"`
	PhysicalBaseline  *KnownGoodBaseline  `json:"physical_baseline,omitempty"`
	SimulatedBaseline *KnownGoodBaseline  `json:"simulated_baseline,omitempty"`
	RecentCaptures    []PassportCapture   `json:"recent_captures"`
	History           []HistorySummary    `json:"history"`
	VerifiedRepairs   []PassportRepair    `json:"verified_repairs"`
	Status            PassportStatus      `json:"status"`
	StatusDetail      string              `json:"status_detail"`
}

type PassportRepository interface {
	GetMeasurement(id int64) (*MeasurementWindow, error)
	SaveKnownGood(record KnownGoodBaseline) (KnownGoodBaseline, error)
	ListKnownGood(profileID string, limit int) ([]KnownGoodBaseline, error)
	LatestKnownGood(profileID string, source BaselineSource, deviceID string) (*KnownGoodBaseline, error)
	LatestKnownGoodOfSource(profileID string, source BaselineSource) (*KnownGoodBaseline, error)
	HasPhysicalBaseline(profileID string) (bool, error)
}
