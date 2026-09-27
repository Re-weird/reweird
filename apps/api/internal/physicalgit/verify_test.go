package physicalgit

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestVerifyStructurallyMatchingStateIsSupported(t *testing.T) {
	profile := &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}}, Connections: []domain.ProfileConnection{{ID: "conn1", Role: "ECHO", GPIO: intPtr(18), Target: "ECHO"}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", ProjectID: "project-a", ProfileSnapshot: profile}
	result := Verify(target, VerifyInput{CurrentProfile: profile})
	if result.Components.Status != domain.VerifySupported || result.Circuit.Status != domain.VerifySupported {
		t.Fatalf("components=%#v circuit=%#v", result.Components, result.Circuit)
	}
}

func TestVerifyStructurallyMismatchingStateIsNotSupported(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", ProfileSnapshot: &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}}}}
	current := &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}, {ID: "c2", Name: "SG90 Servo"}}}
	result := Verify(target, VerifyInput{CurrentProfile: current})
	if result.Components.Status != domain.VerifyNotSupported {
		t.Fatalf("components = %#v", result.Components)
	}
	if result.Overall != domain.VerifyNotSupported {
		t.Fatalf("overall = %s, want NOT_SUPPORTED", result.Overall)
	}
}

func TestVerifyElectricalValidComparisonSupported(t *testing.T) {
	profile := domain.ProjectProfile{
		Confirmed: true,
		Probes:    []domain.ProbeConfiguration{{Probe: "ECHO", Expected: domain.ExpectedSignal{Required: true}}},
	}
	targetMeasurement := &domain.MeasurementWindow{ID: 1, Analysis: domain.AnalysisResult{ProfileID: profile.ID, Probes: []domain.DerivedFacts{{Probe: "ECHO", MissingExpectedActivity: false}}}}
	currentMeasurement := &domain.MeasurementWindow{ID: 2, Analysis: domain.AnalysisResult{ProfileID: profile.ID, Probes: []domain.DerivedFacts{{Probe: "ECHO", MissingExpectedActivity: false}}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", MeasurementID: int64Ptr(1)}
	result := Verify(target, VerifyInput{CurrentProfile: &profile, TargetMeasurement: targetMeasurement, CurrentMeasurement: currentMeasurement})
	if result.Electrical.Status != domain.VerifySupported {
		t.Fatalf("electrical = %#v", result.Electrical)
	}
}

func TestVerifyElectricalWorseIsNotSupported(t *testing.T) {
	profile := domain.ProjectProfile{
		Confirmed: true,
		Probes:    []domain.ProbeConfiguration{{Probe: "ECHO", Expected: domain.ExpectedSignal{Required: true}}},
	}
	targetMeasurement := &domain.MeasurementWindow{ID: 1, Analysis: domain.AnalysisResult{ProfileID: profile.ID, Probes: []domain.DerivedFacts{{Probe: "ECHO", MissingExpectedActivity: false}}}}
	currentMeasurement := &domain.MeasurementWindow{ID: 2, Analysis: domain.AnalysisResult{ProfileID: profile.ID, Probes: []domain.DerivedFacts{{Probe: "ECHO", MissingExpectedActivity: true}}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", MeasurementID: int64Ptr(1)}
	result := Verify(target, VerifyInput{CurrentProfile: &profile, TargetMeasurement: targetMeasurement, CurrentMeasurement: currentMeasurement})
	if result.Electrical.Status != domain.VerifyNotSupported {
		t.Fatalf("electrical = %#v", result.Electrical)
	}
}

func TestVerifyElectricalMissingNewMeasurementIsUnavailable(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", MeasurementID: int64Ptr(1)}
	targetMeasurement := &domain.MeasurementWindow{ID: 1}
	result := Verify(target, VerifyInput{TargetMeasurement: targetMeasurement})
	if result.Electrical.Status != domain.VerifyUnavailable {
		t.Fatalf("electrical = %#v", result.Electrical)
	}
}

func TestVerifyElectricalMissingTargetMeasurementIsUnavailable(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", MeasurementID: int64Ptr(1)}
	current := &domain.MeasurementWindow{ID: 2}
	result := Verify(target, VerifyInput{CurrentMeasurement: current})
	if result.Electrical.Status != domain.VerifyUnavailable {
		t.Fatalf("electrical = %#v", result.Electrical)
	}
}

func TestVerifyVisualUnavailableWhenOnlyOneSideHasImage(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", Image: &domain.ProjectMedia{SHA256: "abc"}}
	result := Verify(target, VerifyInput{})
	if result.Visual.Status != domain.VerifyUnavailable {
		t.Fatalf("visual = %#v", result.Visual)
	}
}

func TestVerifyVisualNeverReturnsNotSupported(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", Image: &domain.ProjectMedia{SHA256: "abc"}}
	result := Verify(target, VerifyInput{ObservationImage: &domain.ProjectMedia{SHA256: "different"}})
	if result.Visual.Status == domain.VerifyNotSupported {
		t.Fatalf("visual must never be NOT_SUPPORTED from raw bytes alone: %#v", result.Visual)
	}
	if result.Visual.Status != domain.VerifyInconclusive {
		t.Fatalf("visual = %#v, want INCONCLUSIVE", result.Visual)
	}
}

func TestVerifySoftwareNotCapturedNeverBlocksOverall(t *testing.T) {
	profile := &domain.ProjectProfile{}
	target := domain.PhysicalCommit{ID: "pcommit-a", ProfileSnapshot: profile}
	result := Verify(target, VerifyInput{CurrentProfile: profile})
	if result.Software.Status != domain.VerifyNotCaptured {
		t.Fatalf("software = %#v", result.Software)
	}
}

func TestVerifyPartialEvidenceOverallSupportedWhenNoneContradict(t *testing.T) {
	profile := &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", ProfileSnapshot: profile}
	// Only structural evidence is available; electrical/visual/software are
	// all NOT_CAPTURED/UNAVAILABLE, but nothing contradicts restoration.
	result := Verify(target, VerifyInput{CurrentProfile: profile})
	if result.Overall != domain.VerifySupported {
		t.Fatalf("overall = %s, want SUPPORTED (matches example: partial evidence still supports)", result.Overall)
	}
}

func TestVerifyNoEvidenceAtAllIsInconclusiveOverall(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a"}
	result := Verify(target, VerifyInput{})
	if result.Overall != domain.VerifyInconclusive {
		t.Fatalf("overall = %s, want INCONCLUSIVE", result.Overall)
	}
}
