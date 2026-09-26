package domain

const (
	ProvenanceUser                   Provenance = "USER"
	ProvenanceProjectProfile         Provenance = "PROJECT_PROFILE"
	ProvenanceComponentSpecification Provenance = "COMPONENT_SPECIFICATION"
)

type HistoryStatus string

const (
	HistoryOpen           HistoryStatus = "OPEN"
	HistoryTesting        HistoryStatus = "TESTING"
	HistoryWaitingForUser HistoryStatus = "WAITING_FOR_USER"
	HistoryVerifying      HistoryStatus = "VERIFYING"
	HistoryResolved       HistoryStatus = "RESOLVED"
	HistoryImproved       HistoryStatus = "IMPROVED"
	HistoryUnresolved     HistoryStatus = "UNRESOLVED"
	HistoryCancelled      HistoryStatus = "CANCELLED"
	HistoryInconclusive   HistoryStatus = "INCONCLUSIVE"
)

type HistoryEvent struct {
	ID          string     `json:"id"`
	TimestampMS int64      `json:"timestamp_ms"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	Provenance  Provenance `json:"provenance"`
	WindowID    int64      `json:"window_id,omitempty"`
}

type HistorySummary struct {
	ID              string        `json:"id"`
	ProjectID       string        `json:"project_id"`
	ProjectName     string        `json:"project_name"`
	SessionID       string        `json:"session_id"`
	ProfileID       string        `json:"profile_id"`
	ProfileVersion  int           `json:"profile_version"`
	TelemetrySource string        `json:"telemetry_source,omitempty"`
	OriginalProblem string        `json:"original_problem"`
	Status          HistoryStatus `json:"status"`
	StartedAtMS     int64         `json:"started_at_ms"`
	EndedAtMS       int64         `json:"ended_at_ms,omitempty"`
}

type HistoryDetail struct {
	Summary  HistorySummary     `json:"summary"`
	Timeline []HistoryEvent     `json:"timeline"`
	Workflow DiagnosticWorkflow `json:"workflow"`
}

type HistoryRepository interface {
	TestWorkflowRepository
	ListTestWorkflows(limit, offset int) ([]DiagnosticWorkflow, error)
}
