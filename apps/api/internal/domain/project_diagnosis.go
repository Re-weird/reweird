package domain

// ProjectDiagnosisStatus distinguishes a real diagnosis from an honest
// admission that there isn't enough electrical evidence yet -- the second
// case must never be disguised as the first.
type ProjectDiagnosisStatus string

const (
	ProjectDiagnosisComplete             ProjectDiagnosisStatus = "DIAGNOSED"
	ProjectDiagnosisInsufficientEvidence ProjectDiagnosisStatus = "INSUFFICIENT_EVIDENCE"
)

// ProjectDiagnosis is the per-project counterpart to the single global
// Session used by the built-in demo/simulator loop. It is never persisted
// separately: it is recomputed on demand from whatever is currently
// persisted for the project (its ProjectProfile, MeasurementWindows, and
// Physical Git history) -- the same "derive, don't cache" pattern already
// used by Physical Restore and Verify.
type ProjectDiagnosis struct {
	ProjectID     string                 `json:"project_id"`
	Status        ProjectDiagnosisStatus `json:"status"`
	Evidence      Evidence               `json:"evidence"`
	Diagnosis     Diagnosis              `json:"diagnosis"`
	MeasurementID *int64                 `json:"measurement_id,omitempty"`
	// ReferenceMeasurementID, when present, is the earlier of the two most
	// recent measurements -- the "before" side of a before/after comparison
	// (e.g. before vs. after a repair attempt). Nil when fewer than two
	// measurements exist yet.
	ReferenceMeasurementID *int64 `json:"reference_measurement_id,omitempty"`
	GeneratedAtMS          int64  `json:"generated_at_ms"`
}
