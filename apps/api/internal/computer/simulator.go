package computer

import (
	"context"
	"fmt"
	"time"
)

const (
	ScenarioHealthy           = "HEALTHY_COMPUTER"
	ScenarioPortConflict      = "PORT_CONFLICT"
	ScenarioHighMemory        = "HIGH_MEMORY"
	ScenarioHighDisk          = "HIGH_DISK"
	ScenarioServiceStopped    = "SERVICE_STOPPED"
	ScenarioLocalUnreachable  = "LOCAL_SERVICE_UNREACHABLE"
	ScenarioDependencyMissing = "DEPENDENCY_MISSING"
	ScenarioVersionMismatch   = "VERSION_MISMATCH"
)

type Scenario struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func Scenarios() []Scenario {
	return []Scenario{
		{ScenarioHealthy, "Healthy computer", "Resources and explicitly expected local services are within configured limits."},
		{ScenarioPortConflict, "Port conflict", "Requested development port is already owned by another process."},
		{ScenarioHighMemory, "High memory", "Memory use is above the configured threshold."},
		{ScenarioHighDisk, "High disk", "A local disk is above the configured usage threshold."},
		{ScenarioServiceStopped, "Service stopped", "An explicitly expected service is stopped."},
		{ScenarioLocalUnreachable, "Local service unreachable", "An expected localhost endpoint did not accept a connection."},
		{ScenarioDependencyMissing, "Dependency missing", "An explicitly required development dependency is absent."},
		{ScenarioVersionMismatch, "Version mismatch", "An observed tool version differs from an explicitly expected version."},
	}
}

type Simulator struct{ ScenarioID string }

func (source Simulator) Name() string { return "computer-simulator" }
func (source Simulator) Collect(_ context.Context, _ Expectations) (Snapshot, error) {
	snapshot, _, err := Simulate(source.ScenarioID)
	return snapshot, err
}

func Simulate(id string) (Snapshot, Expectations, error) {
	snapshot := Snapshot{SchemaVersion: SchemaVersion, DeviceID: "simulated-computer", TimestampMS: time.Now().UTC().UnixMilli(), Source: "simulator", System: System{OS: "Windows (simulated)", Architecture: "amd64", UptimeSeconds: 86_400, CPUPercent: 21, MemoryTotalMB: 16_384, MemoryUsedMB: 6_553}, Disks: []Disk{{Name: "C:", TotalMB: 500_000, UsedMB: 210_000}}, Processes: []Process{{Name: "node.exe", PID: 8412, CPUSeconds: 120, MemoryMB: 360}}, Ports: []Port{}, Interfaces: []Interface{{Name: "Ethernet", State: "Up"}}, Services: []Service{{Name: "ReWeirdDemoService", State: "Running"}}, Dependencies: []Dependency{{Name: "Node", Available: true, Version: "22.0.0"}, {Name: "Go", Available: true, Version: "1.25.0"}, {Name: "Python", Available: true, Version: "3.13.0"}, {Name: "Git", Available: true, Version: "2.48.0"}}, LocalChecks: []LocalCheck{{Port: 3002, Reachable: true}}}
	expected := Expectations{MemoryThresholdPercent: 90, DiskThresholdPercent: 90}
	switch id {
	case ScenarioHealthy:
	case ScenarioPortConflict:
		snapshot.Ports = []Port{{Address: "127.0.0.1", Port: 3000, PID: 8412, Process: "node.exe", State: "LISTENING"}}
		expected.ExclusivePort = 3000
	case ScenarioHighMemory:
		snapshot.System.MemoryUsedMB = 15_500
	case ScenarioHighDisk:
		snapshot.Disks[0].UsedMB = 476_000
	case ScenarioServiceStopped:
		snapshot.Services[0].State = "Stopped"
		expected.ExpectedService = "ReWeirdDemoService"
	case ScenarioLocalUnreachable:
		snapshot.LocalChecks[0].Reachable = false
		expected.ExpectedLocalPort = 3002
	case ScenarioDependencyMissing:
		snapshot.Dependencies[1].Available = false
		snapshot.Dependencies[1].Version = ""
		expected.RequiredDependency = "Go"
	case ScenarioVersionMismatch:
		expected.RequiredDependency = "Node"
		expected.ExpectedVersion = "20.0.0"
	default:
		return Snapshot{}, Expectations{}, fmt.Errorf("unknown computer scenario %q", id)
	}
	return snapshot, expected, nil
}
