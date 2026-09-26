package computer

import (
	"math"
	"testing"
)

func TestSimulatedComputerRules(t *testing.T) {
	cases := []struct{ scenario, want string }{
		{ScenarioHealthy, ""},
		{ScenarioPortConflict, "PORT_CONFLICT_DETECTED"},
		{ScenarioHighMemory, "HIGH_MEMORY_PRESSURE"},
		{ScenarioHighDisk, "HIGH_DISK_USAGE"},
		{ScenarioServiceStopped, "EXPECTED_SERVICE_NOT_RUNNING"},
		{ScenarioLocalUnreachable, "LOCAL_SERVICE_UNREACHABLE"},
		{ScenarioDependencyMissing, "DEVELOPMENT_DEPENDENCY_MISSING"},
		{ScenarioVersionMismatch, "VERSION_MISMATCH"},
	}
	for _, item := range cases {
		t.Run(item.scenario, func(t *testing.T) {
			snapshot, expected, err := Simulate(item.scenario)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Analyze(snapshot, expected)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Evidence.PhysicalEvidence) != 0 {
				t.Fatal("computer evidence crossed into physical evidence")
			}
			for _, fact := range result.Evidence.ComputerEvidence {
				if fact.Name == "memory_usage" && fact.Provenance != DerivedSystem {
					t.Fatal("computed memory percentage mislabeled as measured")
				}
			}
			if item.want == "" && len(result.Findings) != 0 {
				t.Fatalf("healthy computer findings: %+v", result.Findings)
			}
			if item.want != "" {
				found := false
				for _, finding := range result.Findings {
					if finding.Code == item.want {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s in %+v", item.want, result.Findings)
				}
			}
		})
	}
}

func TestComputerExpectationValidation(t *testing.T) {
	if err := ValidateExpectations(Expectations{ExpectedLocalPort: 65536}); err == nil {
		t.Fatal("invalid port accepted")
	}
	if err := ValidateExpectations(Expectations{MemoryThresholdPercent: math.NaN()}); err == nil {
		t.Fatal("NaN accepted")
	}
	snapshot, expected, _ := Simulate(ScenarioHealthy)
	snapshot.SchemaVersion = 2
	if _, err := Analyze(snapshot, expected); err == nil {
		t.Fatal("unsupported version accepted")
	}
}
