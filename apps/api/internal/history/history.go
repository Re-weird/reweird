package history

import (
	"fmt"
	"sort"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func Summary(workflow domain.DiagnosticWorkflow) domain.HistorySummary {
	projectName := workflow.ProjectID
	if workflow.ProfileSnapshot != nil {
		projectName = workflow.ProfileSnapshot.ProjectName
	}
	if projectName == "" {
		projectName = workflow.ProfileID
	}
	status := Status(workflow)
	source := ""
	for _, window := range []*domain.MeasurementWindow{workflow.Baseline, workflow.During, workflow.After} {
		if window != nil {
			source = window.Source
			break
		}
	}
	summary := domain.HistorySummary{
		ID: workflow.ID, ProjectID: workflow.ProjectID, ProjectName: projectName,
		SessionID: workflow.SessionID, ProfileID: workflow.ProfileID, ProfileVersion: workflow.ProfileVersion,
		TelemetrySource: source, OriginalProblem: workflow.Plan.Recommendation.Reason,
		Status: status, StartedAtMS: workflow.CreatedAtMS,
	}
	if status == domain.HistoryResolved || status == domain.HistoryImproved || status == domain.HistoryUnresolved || status == domain.HistoryCancelled || status == domain.HistoryInconclusive {
		summary.EndedAtMS = workflow.UpdatedAtMS
	}
	return summary
}

func Status(workflow domain.DiagnosticWorkflow) domain.HistoryStatus {
	switch workflow.Status {
	case domain.TestPlanned, domain.TestLocked:
		return domain.HistoryOpen
	case domain.TestCapturingBaseline, domain.TestReady, domain.TestCapturingTest, domain.TestAnalyzing:
		return domain.HistoryTesting
	case domain.TestWaitingForUser, domain.TestCompleted:
		return domain.HistoryWaitingForUser
	case domain.TestVerifying:
		return domain.HistoryVerifying
	case domain.TestResolved:
		return domain.HistoryResolved
	case domain.TestUnresolved:
		if workflow.Verification != nil && workflow.Verification.Status == "IMPROVED" {
			return domain.HistoryImproved
		}
		return domain.HistoryUnresolved
	case domain.TestCancelled:
		return domain.HistoryCancelled
	case domain.TestInconclusive, domain.TestFailed:
		return domain.HistoryInconclusive
	default:
		return domain.HistoryInconclusive
	}
}

func Detail(workflow domain.DiagnosticWorkflow) domain.HistoryDetail {
	events := []domain.HistoryEvent{{ID: workflow.ID + "-start", TimestampMS: workflow.CreatedAtMS, Kind: "SESSION_STARTED", Description: "Guided diagnostic test planned from structured evidence.", Provenance: domain.ProvenanceGuidedTest}}
	if workflow.Baseline != nil {
		events = append(events, domain.HistoryEvent{ID: workflow.ID + "-before", TimestampMS: workflow.Baseline.CapturedAtMS, Kind: "BEFORE_CAPTURE", Description: fmt.Sprintf("Before measurement window #%d captured.", workflow.Baseline.ID), Provenance: domain.ProvenanceMeasured, WindowID: workflow.Baseline.ID})
	}
	for _, action := range workflow.UserActions {
		events = append(events, domain.HistoryEvent{ID: action.ID, TimestampMS: action.TimestampMS, Kind: "USER_ACTION", Description: action.Description, Provenance: domain.ProvenanceUser})
	}
	if workflow.During != nil {
		events = append(events, domain.HistoryEvent{ID: workflow.ID + "-during", TimestampMS: workflow.During.CapturedAtMS, Kind: "TEST_CAPTURE", Description: fmt.Sprintf("Guided test window #%d captured.", workflow.During.ID), Provenance: domain.ProvenanceMeasured, WindowID: workflow.During.ID})
	}
	if workflow.Result != nil {
		events = append(events, domain.HistoryEvent{ID: workflow.ID + "-result", TimestampMS: workflow.Result.TimestampMS, Kind: "TEST_RESULT", Description: workflow.Result.Result + ": " + workflow.Result.Interpretation, Provenance: domain.ProvenanceDerived})
	}
	if workflow.After != nil {
		events = append(events, domain.HistoryEvent{ID: workflow.ID + "-after", TimestampMS: workflow.After.CapturedAtMS, Kind: "AFTER_CAPTURE", Description: fmt.Sprintf("After measurement window #%d captured.", workflow.After.ID), Provenance: domain.ProvenanceMeasured, WindowID: workflow.After.ID})
	}
	if workflow.Verification != nil {
		events = append(events, domain.HistoryEvent{ID: workflow.ID + "-verify", TimestampMS: workflow.Verification.TimestampMS, Kind: "VERIFY_RESULT", Description: workflow.Verification.Status + ": " + workflow.Verification.Summary, Provenance: domain.ProvenanceDerived})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].TimestampMS < events[j].TimestampMS })
	return domain.HistoryDetail{Summary: Summary(workflow), Timeline: events, Workflow: workflow}
}
