package domain

// EvidenceState is the deterministic outcome of comparing one evidence
// category between two Physical Commits. It intentionally keeps six
// distinct states so that "not captured" and "unavailable" are never
// collapsed into "unchanged".
type EvidenceState string

const (
	EvidenceUnchanged EvidenceState = "UNCHANGED"
	EvidenceChanged   EvidenceState = "CHANGED"
	EvidenceAdded     EvidenceState = "ADDED"
	EvidenceRemoved   EvidenceState = "REMOVED"
	// EvidenceNotCaptured means neither commit captured this evidence type at all.
	EvidenceNotCaptured EvidenceState = "NOT_CAPTURED"
	// EvidenceUnavailable means evidence exists on at least one side but
	// cannot be meaningfully compared (e.g. one side has no ProjectProfile
	// snapshot at all while the other does, or a referenced row failed to
	// resolve).
	EvidenceUnavailable EvidenceState = "UNAVAILABLE"
)

// FieldChange is a single structured before/after fact. Before/After hold
// the actual stored values (including nil for an unset pointer field) --
// never a generated sentence.
type FieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}

type ComponentChange struct {
	ComponentID string        `json:"component_id"`
	Name        string        `json:"name,omitempty"`
	Status      EvidenceState `json:"status"`
	Fields      []FieldChange `json:"fields,omitempty"`
}

type ConnectionChange struct {
	ConnectionID string        `json:"connection_id"`
	Summary      string        `json:"summary,omitempty"`
	Status       EvidenceState `json:"status"`
	Fields       []FieldChange `json:"fields,omitempty"`
}

type ProbeElectricalChange struct {
	Probe  string        `json:"probe"`
	Status EvidenceState `json:"status"`
	Fields []FieldChange `json:"fields,omitempty"`
}

type VisualDiff struct {
	Status      EvidenceState `json:"status"`
	BeforeImage *ProjectMedia `json:"before_image,omitempty"`
	AfterImage  *ProjectMedia `json:"after_image,omitempty"`
}

type ComponentsDiff struct {
	Status  EvidenceState     `json:"status"`
	Changes []ComponentChange `json:"changes,omitempty"`
}

type CircuitDiff struct {
	Status  EvidenceState      `json:"status"`
	Changes []ConnectionChange `json:"changes,omitempty"`
}

type ElectricalDiff struct {
	Status              EvidenceState           `json:"status"`
	BeforeMeasurementID *int64                  `json:"before_measurement_id,omitempty"`
	AfterMeasurementID  *int64                  `json:"after_measurement_id,omitempty"`
	Probes              []ProbeElectricalChange `json:"probes,omitempty"`
}

type SoftwareDiff struct {
	Status EvidenceState `json:"status"`
	Fields []FieldChange `json:"fields,omitempty"`
}

// PhysicalCommitDiff is a deterministic, structured comparison of two
// Physical Commits in the same project. It is computed on demand and never
// persisted. No field here is inferred from pixels, wiring photos, or any
// AI interpretation -- only from the structured evidence each commit
// actually captured.
type PhysicalCommitDiff struct {
	ProjectID  string `json:"project_id"`
	FromCommit string `json:"from_commit"`
	ToCommit   string `json:"to_commit"`

	Visual     VisualDiff     `json:"visual"`
	Components ComponentsDiff `json:"components"`
	Circuit    CircuitDiff    `json:"circuit"`
	Electrical ElectricalDiff `json:"electrical"`
	Software   SoftwareDiff   `json:"software"`
}
