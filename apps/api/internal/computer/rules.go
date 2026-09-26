package computer

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	MeasuredSystem = "MEASURED_SYSTEM"
	DerivedSystem  = "DERIVED_SYSTEM"
)

func Analyze(snapshot Snapshot, expected Expectations) (Analysis, error) {
	if err := validate(snapshot, expected); err != nil {
		return Analysis{}, err
	}
	if expected.MemoryThresholdPercent == 0 {
		expected.MemoryThresholdPercent = 90
	}
	if expected.DiskThresholdPercent == 0 {
		expected.DiskThresholdPercent = 90
	}
	result := Analysis{Snapshot: snapshot, Expectations: expected, Findings: []Finding{}, Evidence: DiagnosticEvidence{PhysicalEvidence: []Fact{}, ComputerEvidence: []Fact{}, SoftwareEvidence: []Fact{}}}
	add := func(name string, value any, unit string) Fact {
		fact := Fact{Name: name, Value: value, Unit: unit, Provenance: MeasuredSystem}
		result.Evidence.ComputerEvidence = append(result.Evidence.ComputerEvidence, fact)
		return fact
	}
	addDerived := func(name string, value any, unit string) Fact {
		fact := Fact{Name: name, Value: value, Unit: unit, Provenance: DerivedSystem}
		result.Evidence.ComputerEvidence = append(result.Evidence.ComputerEvidence, fact)
		return fact
	}
	add("os", snapshot.System.OS, "")
	add("architecture", snapshot.System.Architecture, "")
	add("cpu_usage", snapshot.System.CPUPercent, "%")
	if snapshot.System.MemoryTotalMB > 0 {
		add("memory_total", snapshot.System.MemoryTotalMB, "MB")
		add("memory_used", snapshot.System.MemoryUsedMB, "MB")
		used := float64(snapshot.System.MemoryUsedMB) / float64(snapshot.System.MemoryTotalMB) * 100
		fact := addDerived("memory_usage", used, "%")
		if used > expected.MemoryThresholdPercent {
			result.Findings = append(result.Findings, Finding{Code: "HIGH_MEMORY_PRESSURE", Severity: "warning", Summary: fmt.Sprintf("Memory use is %.1f%%, above the configured %.1f%% threshold.", used, expected.MemoryThresholdPercent), Evidence: []Fact{fact}, NextAction: "Inspect memory-intensive processes; no process will be stopped automatically."})
		}
	}
	for _, disk := range snapshot.Disks {
		if disk.TotalMB == 0 {
			continue
		}
		used := float64(disk.UsedMB) / float64(disk.TotalMB) * 100
		add("disk_total_"+disk.Name, disk.TotalMB, "MB")
		add("disk_used_"+disk.Name, disk.UsedMB, "MB")
		fact := addDerived("disk_usage_"+disk.Name, used, "%")
		if used > expected.DiskThresholdPercent {
			result.Findings = append(result.Findings, Finding{Code: "HIGH_DISK_USAGE", Severity: "warning", Summary: fmt.Sprintf("Disk %s is %.1f%% used, above the configured %.1f%% threshold.", disk.Name, used, expected.DiskThresholdPercent), Evidence: []Fact{fact}, NextAction: "Review disk usage manually; ReWeird will not delete files."})
		}
	}
	for _, port := range snapshot.Ports {
		if expected.ExclusivePort > 0 && port.Port == expected.ExclusivePort && strings.EqualFold(port.State, "LISTENING") {
			facts := []Fact{add("listening_port", port.Port, "TCP"), add("owning_pid", port.PID, "")}
			if port.Process != "" {
				facts = append(facts, add("owning_process", port.Process, ""))
			}
			result.Findings = append(result.Findings, Finding{Code: "PORT_CONFLICT_DETECTED", Severity: "warning", Summary: fmt.Sprintf("Requested local port %d is already listening under PID %d.", port.Port, port.PID), Evidence: facts, NextAction: "Inspect the process using this port; ReWeird will not terminate it."})
			break
		}
	}
	if expected.ExpectedService != "" {
		found := false
		for _, service := range snapshot.Services {
			if strings.EqualFold(service.Name, expected.ExpectedService) {
				found = true
				if !strings.EqualFold(service.State, "running") {
					fact := add("service_state_"+service.Name, service.State, "")
					result.Findings = append(result.Findings, Finding{Code: "EXPECTED_SERVICE_NOT_RUNNING", Severity: "warning", Summary: fmt.Sprintf("Expected service %s is %s.", service.Name, service.State), Evidence: []Fact{fact}, NextAction: "Inspect the service manually; ReWeird will not start it."})
				}
				break
			}
		}
		if !found {
			result.Findings = append(result.Findings, Finding{Code: "SERVICE_EVIDENCE_UNAVAILABLE", Severity: "info", Summary: "The expected service was not present in this bounded snapshot.", Evidence: []Fact{}, NextAction: "Confirm the service name and collect another snapshot."})
		}
	}
	if expected.ExpectedLocalPort > 0 {
		found := false
		for _, check := range snapshot.LocalChecks {
			if check.Port == expected.ExpectedLocalPort {
				found = true
				if !check.Reachable {
					fact := add("localhost_reachable", false, "")
					result.Findings = append(result.Findings, Finding{Code: "LOCAL_SERVICE_UNREACHABLE", Severity: "warning", Summary: fmt.Sprintf("Expected localhost port %d did not accept a connection.", check.Port), Evidence: []Fact{fact}, NextAction: "Inspect the service and its logs manually; no configuration will be changed."})
				}
				break
			}
		}
		if !found {
			result.Findings = append(result.Findings, Finding{Code: "LOCAL_SERVICE_EVIDENCE_UNAVAILABLE", Severity: "info", Summary: "No localhost availability check was recorded for the expected port.", Evidence: []Fact{}, NextAction: "Collect a snapshot with the requested localhost port."})
		}
	}
	if expected.RequiredDependency != "" {
		found := false
		for _, dependency := range snapshot.Dependencies {
			if strings.EqualFold(dependency.Name, expected.RequiredDependency) {
				found = true
				if !dependency.Available {
					fact := Fact{Name: "dependency_available", Value: false, Provenance: MeasuredSystem}
					result.Evidence.SoftwareEvidence = append(result.Evidence.SoftwareEvidence, fact)
					result.Findings = append(result.Findings, Finding{Code: "DEVELOPMENT_DEPENDENCY_MISSING", Severity: "warning", Summary: dependency.Name + " was not found on PATH.", Evidence: []Fact{fact}, NextAction: "Verify the project's tool requirements and installation manually."})
				} else if expected.ExpectedVersion != "" && dependency.Version != "" && dependency.Version != expected.ExpectedVersion {
					fact := Fact{Name: "dependency_version", Value: dependency.Version, Provenance: MeasuredSystem}
					result.Evidence.SoftwareEvidence = append(result.Evidence.SoftwareEvidence, fact)
					result.Findings = append(result.Findings, Finding{Code: "VERSION_MISMATCH", Severity: "warning", Summary: fmt.Sprintf("%s version %s differs from the explicitly expected %s.", dependency.Name, dependency.Version, expected.ExpectedVersion), Evidence: []Fact{fact}, NextAction: "Check the project's supported version before changing tools."})
				}
				break
			}
		}
		if !found {
			result.Findings = append(result.Findings, Finding{Code: "DEPENDENCY_EVIDENCE_UNAVAILABLE", Severity: "info", Summary: "The required dependency was not included in this snapshot.", Evidence: []Fact{}, NextAction: "Collect another snapshot with the dependency available in the tool inventory."})
		}
	}
	return result, nil
}

func validate(snapshot Snapshot, expected Expectations) error {
	if err := ValidateExpectations(expected); err != nil {
		return err
	}
	if snapshot.SchemaVersion != SchemaVersion {
		return errors.New("unsupported computer snapshot schema version")
	}
	if snapshot.DeviceID == "" || snapshot.TimestampMS <= 0 || snapshot.Source == "" {
		return errors.New("computer snapshot identity, timestamp, and source are required")
	}
	if len(snapshot.Processes) > 50 || len(snapshot.Ports) > 150 || len(snapshot.Services) > 512 || len(snapshot.Disks) > 32 || len(snapshot.Interfaces) > 64 || len(snapshot.Dependencies) > 32 {
		return errors.New("computer snapshot exceeds bounded collection limits")
	}
	if snapshot.System.CPUPercent < 0 || snapshot.System.CPUPercent > 100 || math.IsNaN(snapshot.System.CPUPercent) || snapshot.System.MemoryUsedMB > snapshot.System.MemoryTotalMB {
		return errors.New("invalid system resource values")
	}
	for _, disk := range snapshot.Disks {
		if disk.UsedMB > disk.TotalMB {
			return errors.New("invalid disk resource values")
		}
	}
	for _, port := range snapshot.Ports {
		if port.Port < 1 || port.Port > 65535 || port.PID < 0 {
			return errors.New("invalid listening port")
		}
	}
	return nil
}

func ValidateExpectations(expected Expectations) error {
	if expected.ExclusivePort < 0 || expected.ExclusivePort > 65535 || expected.ExpectedLocalPort < 0 || expected.ExpectedLocalPort > 65535 {
		return errors.New("expected port must be 1-65535 or omitted")
	}
	if expected.MemoryThresholdPercent < 0 || expected.MemoryThresholdPercent > 100 || expected.DiskThresholdPercent < 0 || expected.DiskThresholdPercent > 100 || math.IsNaN(expected.MemoryThresholdPercent) || math.IsNaN(expected.DiskThresholdPercent) {
		return errors.New("thresholds must be between 0 and 100")
	}
	if len(expected.ExpectedService) > 80 || len(expected.RequiredDependency) > 80 || len(expected.ExpectedVersion) > 40 {
		return errors.New("expectation text is too long")
	}
	return nil
}
