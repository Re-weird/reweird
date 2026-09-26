package computer

import "context"

const SchemaVersion = 1

type System struct {
	OS            string  `json:"os"`
	Architecture  string  `json:"architecture"`
	UptimeSeconds uint64  `json:"uptime_seconds"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryTotalMB uint64  `json:"memory_total_mb"`
	MemoryUsedMB  uint64  `json:"memory_used_mb"`
}

type Disk struct {
	Name    string `json:"name"`
	TotalMB uint64 `json:"total_mb"`
	UsedMB  uint64 `json:"used_mb"`
}
type Process struct {
	Name       string  `json:"name"`
	PID        int     `json:"pid"`
	CPUSeconds float64 `json:"cpu_seconds"`
	MemoryMB   uint64  `json:"memory_mb"`
}
type Port struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Process string `json:"process,omitempty"`
	State   string `json:"state"`
}
type Interface struct {
	Name  string `json:"name"`
	State string `json:"state"`
}
type Service struct {
	Name  string `json:"name"`
	State string `json:"state"`
}
type Dependency struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
}
type LocalCheck struct {
	Port      int  `json:"port"`
	Reachable bool `json:"reachable"`
}

type Snapshot struct {
	SchemaVersion int          `json:"schema_version"`
	DeviceID      string       `json:"device_id"`
	TimestampMS   int64        `json:"timestamp_ms"`
	Source        string       `json:"source"`
	System        System       `json:"system"`
	Disks         []Disk       `json:"disks"`
	Processes     []Process    `json:"processes"`
	Ports         []Port       `json:"ports"`
	Interfaces    []Interface  `json:"interfaces"`
	Services      []Service    `json:"services"`
	Dependencies  []Dependency `json:"development_environment"`
	LocalChecks   []LocalCheck `json:"local_checks"`
}

type Expectations struct {
	ExclusivePort          int     `json:"exclusive_port,omitempty"`
	ExpectedService        string  `json:"expected_service,omitempty"`
	ExpectedLocalPort      int     `json:"expected_local_port,omitempty"`
	RequiredDependency     string  `json:"required_dependency,omitempty"`
	ExpectedVersion        string  `json:"expected_version,omitempty"`
	MemoryThresholdPercent float64 `json:"memory_threshold_percent,omitempty"`
	DiskThresholdPercent   float64 `json:"disk_threshold_percent,omitempty"`
}

type Fact struct {
	Name       string `json:"name"`
	Value      any    `json:"value"`
	Unit       string `json:"unit,omitempty"`
	Provenance string `json:"provenance"`
}
type Finding struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Summary    string `json:"summary"`
	Evidence   []Fact `json:"evidence"`
	NextAction string `json:"next_action"`
}
type Analysis struct {
	Snapshot     Snapshot           `json:"snapshot"`
	Expectations Expectations       `json:"expectations"`
	Findings     []Finding          `json:"findings"`
	Evidence     DiagnosticEvidence `json:"evidence"`
}

// Domain-separated boundary for future reasoning, not an AI reasoning result.
type DiagnosticEvidence struct {
	PhysicalEvidence []Fact `json:"physical_evidence"`
	ComputerEvidence []Fact `json:"computer_evidence"`
	SoftwareEvidence []Fact `json:"software_evidence"`
}

type TelemetrySource interface {
	Name() string
	Collect(ctx context.Context, expectations Expectations) (Snapshot, error)
}

func PlatformCollector() TelemetrySource { return WindowsCollector{} }
