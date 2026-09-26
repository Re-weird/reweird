package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/reports"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/vision"
)

func TestHistoryAndDetailedReportPersistAfterRestart(t *testing.T) {
	app, repository, databasePath, _ := testAppWithVision(t, vision.SkippedAnalyzer{})
	response := doJSON(t, app, http.MethodGet, "/api/v1/tests/recommendation", nil)
	var recommendation domain.TestRecommendation
	decodeBody(t, response, &recommendation)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests", recommendation)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status %d", response.StatusCode)
	}
	var workflow domain.DiagnosticWorkflow
	decodeBody(t, response, &workflow)
	if workflow.ProfileSnapshot == nil || workflow.ProfileSnapshot.Version != workflow.ProfileVersion {
		t.Fatalf("profile snapshot missing: %+v", workflow)
	}
	base := "/api/v1/tests/" + workflow.ID
	response = doJSON(t, app, http.MethodPost, base+"/start", nil)
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, base+"/actions", map[string]string{"description": "Gently moved the connector during the capture."})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("user action status %d: %s", response.StatusCode, readBody(t, response))
	}
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, base+"/capture", nil)
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, base+"/actions", map[string]string{"description": "Reseated the connector before re-measurement."})
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, base+"/remeasure", nil)
	decodeBody(t, response, &workflow)
	if workflow.Status != domain.TestResolved {
		t.Fatalf("status = %s", workflow.Status)
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/history?project_id=ultrasonic-demo&status=RESOLVED&sort=newest", nil)
	var listing struct {
		Items []domain.HistorySummary `json:"items"`
		Count int                     `json:"count"`
	}
	decodeBody(t, response, &listing)
	if listing.Count != 1 || listing.Items[0].ProfileVersion != workflow.ProfileVersion {
		t.Fatalf("listing = %+v", listing)
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/history?status=CANCELLED", nil)
	decodeBody(t, response, &listing)
	if listing.Count != 0 {
		t.Fatalf("status filter = %+v", listing)
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/history/"+workflow.ID, nil)
	var detail domain.HistoryDetail
	decodeBody(t, response, &detail)
	if len(detail.Timeline) < 7 || detail.Summary.Status != domain.HistoryResolved || len(detail.Workflow.UserActions) != 2 {
		t.Fatalf("detail = %+v", detail)
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/reports/"+workflow.ID, nil)
	var report reports.DetailedReport
	decodeBody(t, response, &report)
	if report.FinalStatus != domain.HistoryResolved || report.BeforeWindowID == 0 || report.AfterWindowID == 0 || report.VerifyResult == nil || len(report.MeasuredEvidence) == 0 || len(report.DerivedEvidence) == 0 {
		t.Fatalf("report = %+v", report)
	}
	response = doJSON(t, app, http.MethodGet, "/api/v1/reports/"+workflow.ID+"?format=md", nil)
	markdown := readBody(t, response)
	if !strings.Contains(markdown, "# ReWeird diagnostic report") || !strings.Contains(markdown, "Reseated the connector") || !strings.Contains(markdown, "## Provenance") {
		t.Fatalf("markdown = %q", markdown)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := NewApp(diagnostics.NewEngine(signalanalysis.New()), reopened, simulator.NewUltrasonicSource(), "ultrasonic-demo", ProjectServices{}, false)
	response = doJSON(t, restarted, http.MethodGet, "/api/v1/history/"+workflow.ID, nil)
	decodeBody(t, response, &detail)
	if detail.Summary.Status != domain.HistoryResolved || len(detail.Workflow.UserActions) != 2 {
		t.Fatalf("restarted detail = %+v", detail)
	}
}

func TestHistoryRejectsSecretActionAndInvalidFilter(t *testing.T) {
	app, _ := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/history?status=NOT_A_STATUS", nil)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
	response = doJSON(t, app, http.MethodGet, "/api/v1/tests/recommendation", nil)
	var rec domain.TestRecommendation
	decodeBody(t, response, &rec)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests", rec)
	var workflow domain.DiagnosticWorkflow
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/start", nil)
	_ = readBody(t, response)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/actions", map[string]string{"description": "api_key=topsecretvalue123"})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("secret action status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
}
