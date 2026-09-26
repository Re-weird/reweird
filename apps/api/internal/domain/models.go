package domain

type Stage string

const (
	StageDetect   Stage = "detect"
	StageDiagnose Stage = "diagnose"
	StageTest     Stage = "test"
	StageRepair   Stage = "repair"
	StageVerify   Stage = "verify"
)

type ProbeReading struct {
	Probe    string    `json:"probe"`
	Role     string    `json:"role"`
	Value    *float64  `json:"value"`
	Unit     string    `json:"unit"`
	Status   string    `json:"status"`
	Dropouts int       `json:"dropouts"`
	Samples  []float64 `json:"samples"`
}

type RuleResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Evidence struct {
	Probe       string         `json:"probe"`
	Role        string         `json:"role"`
	Expected    map[string]any `json:"expected"`
	Observed    map[string]any `json:"observed"`
	Baseline    map[string]any `json:"baseline"`
	RuleResults []RuleResult   `json:"rule_results"`
}

type Diagnosis struct {
	Headline      string   `json:"headline"`
	Summary       string   `json:"summary"`
	PossibleCauses []string `json:"possible_causes"`
	Confidence    float64  `json:"confidence"`
	NextTest      string   `json:"next_test"`
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
	Probes            []ProbeReading      `json:"probes"`
	Evidence          Evidence            `json:"evidence"`
	Diagnosis         Diagnosis           `json:"diagnosis"`
	Before            MeasurementSummary  `json:"before"`
	After             *MeasurementSummary `json:"after,omitempty"`
	Timeline          []TimelineEvent     `json:"timeline"`
}

type TelemetrySource interface {
	Snapshot(stage Stage) []ProbeReading
}

type SessionRepository interface {
	SaveSession(session Session) error
	LatestSession() (*Session, error)
}
