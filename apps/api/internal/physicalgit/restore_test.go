package physicalgit

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestRestoreNoSourceIsUnavailableEverywhere(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-target", ProjectID: "project-a", DisplayID: "HW-001"}
	plan := Restore(target, nil, nil, nil, nil, nil)
	if plan.HasSource {
		t.Fatalf("HasSource = true, want false")
	}
	for name, section := range map[string]domain.RestoreSection{
		"components": plan.Components, "circuit": plan.Circuit, "electrical": plan.Electrical, "visual": plan.Visual, "software": plan.Software,
	} {
		if section.Status != domain.RestoreUnavailable {
			t.Fatalf("%s status = %s, want UNAVAILABLE", name, section.Status)
		}
	}
}

func TestRestoreIdenticalTargetAndSourceIsAllMatch(t *testing.T) {
	profile := &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}}, Connections: []domain.ProfileConnection{{ID: "conn1", Role: "ECHO", GPIO: intPtr(18), Target: "ECHO"}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", ProjectID: "project-a", DisplayID: "HW-001", ProfileSnapshot: profile}
	source := domain.PhysicalCommit{ID: "pcommit-b", ProjectID: "project-a", DisplayID: "HW-002", ProfileSnapshot: profile}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Components.Status != domain.RestoreMatch {
		t.Fatalf("components = %#v", plan.Components)
	}
	if plan.Circuit.Status != domain.RestoreMatch {
		t.Fatalf("circuit = %#v", plan.Circuit)
	}
}

func TestRestoreComponentActionRequiredWhenMissingFromSource(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-003", ProfileSnapshot: &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "c1", Name: "HC-SR04"}}}}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-007", ProfileSnapshot: &domain.ProjectProfile{}}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Components.Status != domain.RestoreActionRequired || len(plan.Components.Actions) != 1 {
		t.Fatalf("components = %#v", plan.Components)
	}
	action := plan.Components.Actions[0]
	if action.Status != domain.RestoreActionRequired || action.Description == "" {
		t.Fatalf("action = %#v", action)
	}
}

func TestRestoreComponentActionRequiredWhenExtraInSource(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-003", ProfileSnapshot: &domain.ProjectProfile{}}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-007", ProfileSnapshot: &domain.ProjectProfile{Components: []domain.ComponentSpecification{{ID: "servo-1", Name: "SG90 Servo"}}}}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Components.Status != domain.RestoreActionRequired || len(plan.Components.Actions) != 1 {
		t.Fatalf("components = %#v", plan.Components)
	}
	action := plan.Components.Actions[0]
	if action.CurrentValue != "SG90 Servo" || action.TargetValue != "not present" {
		t.Fatalf("action = %#v", action)
	}
}

func TestRestoreCircuitGPIOMismatchProducesTargetedGuidance(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-003", ProfileSnapshot: &domain.ProjectProfile{Connections: []domain.ProfileConnection{{ID: "echo", Role: "ECHO", GPIO: intPtr(18), Target: "ECHO"}}}}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-007", ProfileSnapshot: &domain.ProjectProfile{Connections: []domain.ProfileConnection{{ID: "echo", Role: "ECHO", GPIO: intPtr(19), Target: "ECHO"}}}}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Circuit.Status != domain.RestoreActionRequired || len(plan.Circuit.Actions) != 1 {
		t.Fatalf("circuit = %#v", plan.Circuit)
	}
	action := plan.Circuit.Actions[0]
	if action.Description != "Restore ECHO connection to GPIO18." {
		t.Fatalf("description = %q", action.Description)
	}
}

func TestRestoreElectricalIsAlwaysVerifyRequiredNeverAction(t *testing.T) {
	measurement := &domain.MeasurementWindow{ID: 1, Analysis: domain.AnalysisResult{Probes: []domain.DerivedFacts{{Probe: "ECHO", AverageVoltage: float64Ptr(3.3), Stable: true}}}}
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-003", MeasurementID: int64Ptr(1)}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-007"}
	plan := Restore(target, &source, measurement, nil, nil, nil)
	if plan.Electrical.Status != domain.RestoreVerifyRequired {
		t.Fatalf("electrical status = %s, want VERIFY_REQUIRED", plan.Electrical.Status)
	}
	for _, action := range plan.Electrical.Actions {
		if action.Status == domain.RestoreActionRequired {
			t.Fatalf("electrical must never produce ACTION_REQUIRED: %#v", action)
		}
	}
}

func TestRestoreElectricalNotCapturedWhenNeitherSideHasMeasurement(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-001"}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-002"}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Electrical.Status != domain.RestoreNotCaptured {
		t.Fatalf("electrical status = %s, want NOT_CAPTURED", plan.Electrical.Status)
	}
}

func TestRestoreVisualNeverProducesActionRequired(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-001", Image: &domain.ProjectMedia{SHA256: "abc"}}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-002", Image: &domain.ProjectMedia{SHA256: "def"}}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Visual.Status != domain.RestoreVerifyRequired {
		t.Fatalf("visual status = %s, want VERIFY_REQUIRED", plan.Visual.Status)
	}
	for _, action := range plan.Visual.Actions {
		if action.Status == domain.RestoreActionRequired {
			t.Fatalf("visual must never produce ACTION_REQUIRED: %#v", action)
		}
	}
}

func TestRestoreVisualSemanticNeverInvokesGeminiAndIsMarkedAIInterpreted(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-001"}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-002"}
	targetVision := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "HC-SR04"}}}}
	sourceVision := &domain.PhysicalCommitVisionAnalysis{Analysis: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "HC-SR04"}, {Name: "SG90 Servo"}}}}
	plan := Restore(target, &source, nil, nil, targetVision, sourceVision)
	found := false
	for _, action := range plan.Visual.Actions {
		if action.AIInterpreted && action.Title == "SG90 Servo" {
			found = true
			if action.Status == domain.RestoreActionRequired {
				t.Fatalf("AI-interpreted action must not be ACTION_REQUIRED: %#v", action)
			}
		}
	}
	if !found {
		t.Fatalf("expected an AI-interpreted action for SG90 Servo, got %#v", plan.Visual.Actions)
	}
}

func TestRestoreSoftwareNotCapturedWhenBothNil(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-001"}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-002"}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Software.Status != domain.RestoreNotCaptured {
		t.Fatalf("software status = %s, want NOT_CAPTURED", plan.Software.Status)
	}
}

func TestRestoreSoftwareNeverAutomatesCheckout(t *testing.T) {
	target := domain.PhysicalCommit{ID: "pcommit-a", DisplayID: "HW-001", SoftwareRevision: stringPtr("8ac31f2")}
	source := domain.PhysicalCommit{ID: "pcommit-b", DisplayID: "HW-002", SoftwareRevision: stringPtr("91be002")}
	plan := Restore(target, &source, nil, nil, nil, nil)
	if plan.Software.Status != domain.RestoreActionRequired {
		t.Fatalf("software status = %s", plan.Software.Status)
	}
	for _, action := range plan.Software.Actions {
		if action.Description == "" {
			t.Fatalf("action missing description: %#v", action)
		}
	}
}
