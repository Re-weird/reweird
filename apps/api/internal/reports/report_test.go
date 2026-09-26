package reports

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestBuildReportCarriesDiagnosisAndEvidence(t *testing.T) {
	session := domain.Session{
		ID:          "session-demo",
		ProjectName: "Parking Sensor",
		ProfileID:   "ultrasonic-demo",
		Stage:       domain.StageDiagnose,
		Diagnosis: domain.Diagnosis{
			Headline:       "Missing echo activity",
			Summary:        "No echo pulses detected.",
			PossibleCauses: []string{"Open signal path"},
			Confidence:     0.78,
			NextTest:       "Confirm the probe assignment.",
		},
		Evidence: domain.Evidence{Probe: "P2", Role: "ECHO"},
		Before:   domain.MeasurementSummary{DropoutsPerMinute: 5, Stability: "intermittent"},
	}

	report := BuildReport(session)

	if report.SessionID != session.ID {
		t.Fatalf("expected session id %q, got %q", session.ID, report.SessionID)
	}
	if report.Headline != session.Diagnosis.Headline {
		t.Fatalf("expected headline copied from diagnosis")
	}
	if report.Evidence.Probe != "P2" {
		t.Fatalf("expected evidence copied from session")
	}
	if report.Verify != nil {
		t.Fatalf("expected no verify result outside the VERIFY stage")
	}
}

func TestBuildReportFixedWhenVerifyStageStableWithoutFailure(t *testing.T) {
	after := domain.MeasurementSummary{DropoutsPerMinute: 0, Stability: "stable"}
	session := domain.Session{
		Stage:     domain.StageVerify,
		After:     &after,
		Evidence:  domain.Evidence{RuleResults: []domain.RuleResult{{ID: "unexpected-dropout", Status: "pass"}}},
		Diagnosis: domain.Diagnosis{Headline: "Issue resolved"},
	}

	report := BuildReport(session)

	if report.Verify == nil || *report.Verify != domain.VerifyFixed {
		t.Fatalf("expected fixed verify result, got %v", report.Verify)
	}
}

func TestBuildReportStillFailingWhenVerifyStageUnstable(t *testing.T) {
	after := domain.MeasurementSummary{DropoutsPerMinute: 9, Stability: "intermittent"}
	session := domain.Session{
		Stage: domain.StageVerify,
		After: &after,
	}

	report := BuildReport(session)

	if report.Verify == nil || *report.Verify != domain.VerifyStillFailing {
		t.Fatalf("expected still_failing verify result, got %v", report.Verify)
	}
}

func TestBuildReportUnclearWhenVerifyStageHasNoAfterMeasurement(t *testing.T) {
	session := domain.Session{Stage: domain.StageVerify}

	report := BuildReport(session)

	if report.Verify == nil || *report.Verify != domain.VerifyUnclear {
		t.Fatalf("expected unclear verify result, got %v", report.Verify)
	}
}
