package domain

// CalibrationStatus is the commissioning state of one device + profile
// revision. It only ever advances to CALIBRATED through an explicit user
// confirmation of a physical (REAL_SERIAL) observation.
type CalibrationStatus string

const (
	// NOT_CALIBRATED: no physical capture exists for this profile revision.
	CalibrationNotCalibrated CalibrationStatus = "NOT_CALIBRATED"
	// OBSERVING: physical windows are arriving but not enough consecutive
	// windows have been observed to learn stable behavior.
	CalibrationObserving CalibrationStatus = "OBSERVING"
	// ATTENTION_REQUIRED: the observation violates configured expectations or
	// the capture itself is unreliable. It cannot be saved as Known Good.
	CalibrationAttentionRequired CalibrationStatus = "ATTENTION_REQUIRED"
	// REVIEW_REQUIRED: stable, consistent physical behavior was observed; the
	// user must confirm the circuit is currently operating correctly.
	CalibrationReviewRequired CalibrationStatus = "REVIEW_REQUIRED"
	// CALIBRATED: a compatible physical Known Good exists.
	CalibrationCalibrated CalibrationStatus = "CALIBRATED"
	// BASELINE_INCOMPATIBLE: a Known Good exists for this device, but for a
	// different profile revision or probe mapping.
	CalibrationBaselineIncompatible CalibrationStatus = "BASELINE_INCOMPATIBLE"
	// SIMULATED_SOURCE: the active telemetry is simulated; physical
	// calibration is impossible from this source.
	CalibrationSimulatedSource CalibrationStatus = "SIMULATED_SOURCE"
)

// ObservedSummary aggregates recent physical windows for one probe. Every
// value is derived from stored raw captures; nothing here is configured.
type ObservedSummary struct {
	Windows             int      `json:"windows"`
	StableWindows       int      `json:"stable_windows"`
	UnreliableWindows   int      `json:"unreliable_windows"`
	MaxDropouts         int      `json:"max_dropouts"`
	MinVoltage          *float64 `json:"min_voltage,omitempty"`
	AverageVoltage      *float64 `json:"average_voltage,omitempty"`
	MaxVoltage          *float64 `json:"max_voltage,omitempty"`
	MaxVoltageVariation *float64 `json:"max_voltage_variation,omitempty"`
	MinFrequencyHz      *float64 `json:"min_frequency_hz,omitempty"`
	AverageFrequencyHz  *float64 `json:"average_frequency_hz,omitempty"`
	MaxFrequencyHz      *float64 `json:"max_frequency_hz,omitempty"`
	MinPulseWidthUS     *float64 `json:"min_pulse_width_us,omitempty"`
	AveragePulseWidthUS *float64 `json:"average_pulse_width_us,omitempty"`
	MaxPulseWidthUS     *float64 `json:"max_pulse_width_us,omitempty"`
	ActiveWindows       int      `json:"active_windows"`
	CaptureIssues       []string `json:"capture_issues,omitempty"`
}

// CalibrationProbe keeps the three evidence layers visibly separate.
type CalibrationProbe struct {
	Probe     string           `json:"probe"`
	Role      string           `json:"role"`
	Mode      ProbeMode        `json:"mode"`
	Required  bool             `json:"required"`
	Expected  ExpectedSignal   `json:"expected"`
	Observed  ObservedSummary  `json:"observed"`
	KnownGood *TrustedBaseline `json:"known_good,omitempty"`
	Issues    []string         `json:"issues,omitempty"`
}

type CalibrationState struct {
	Status          CalibrationStatus     `json:"status"`
	Detail          string                `json:"detail"`
	ProfileID       string                `json:"profile_id"`
	ProfileVersion  int                   `json:"profile_version"`
	DeviceID        string                `json:"device_id,omitempty"`
	Provenance      MeasurementProvenance `json:"provenance,omitempty"`
	WindowsObserved int                   `json:"windows_observed"`
	WindowsRequired int                   `json:"windows_required"`
	// CandidateMeasurementID is the newest window of the observed run; the
	// user confirms against this exact evidence.
	CandidateMeasurementID int64              `json:"candidate_measurement_id,omitempty"`
	FirstMeasurementID     int64              `json:"first_measurement_id,omitempty"`
	CanSaveKnownGood       bool               `json:"can_save_known_good"`
	Blockers               []string           `json:"blockers,omitempty"`
	Probes                 []CalibrationProbe `json:"probes"`
	KnownGood              *KnownGoodBaseline `json:"known_good,omitempty"`
	ProbeMappingHash       string             `json:"probe_mapping_hash"`
}
