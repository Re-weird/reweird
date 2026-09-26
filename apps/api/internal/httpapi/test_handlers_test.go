package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/vision"
)

func TestGuidedWorkflowAcrossSimulatorScenarios(t *testing.T) {
	cases := []struct {
		scenario string
		kind     domain.TestType
	}{
		{simulator.ScenarioHealthy, domain.TestBaselineComparison},
		{simulator.ScenarioDeadSignal, domain.TestSignalActivity},
		{simulator.ScenarioLowVoltage, domain.TestPowerRailStability},
		{simulator.ScenarioUnstablePower, domain.TestPowerRailStability},
		{simulator.ScenarioMissingPulses, domain.TestMovementCorrelation},
		{simulator.ScenarioIntermittent, domain.TestMovementCorrelation},
		{simulator.ScenarioSimultaneousDropouts, domain.TestSimultaneousDropout},
		{simulator.ScenarioTimingDrift, domain.TestFrequencyTiming},
		{simulator.ScenarioSoftwareChange, domain.TestSignalActivity},
	}
	for _, item := range cases {
		t.Run(item.scenario, func(t *testing.T) {
			app, repository := testApp(t)
			response := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]string{"scenario_id": item.scenario})
			if response.StatusCode != http.StatusOK {
				t.Fatalf("scenario status %d: %s", response.StatusCode, readBody(t, response))
			}
			_ = readBody(t, response)
			response = doJSON(t, app, http.MethodGet, "/api/v1/tests/recommendation", nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("recommendation status %d: %s", response.StatusCode, readBody(t, response))
			}
			var recommended domain.TestRecommendation
			decodeBody(t, response, &recommended)
			if recommended.TestType != item.kind {
				t.Fatalf("recommendation type = %s, want %s", recommended.TestType, item.kind)
			}
			response = doJSON(t, app, http.MethodPost, "/api/v1/tests", recommended)
			if response.StatusCode != http.StatusCreated {
				t.Fatalf("plan status %d: %s", response.StatusCode, readBody(t, response))
			}
			var workflow domain.DiagnosticWorkflow
			decodeBody(t, response, &workflow)
			if workflow.Status != domain.TestPlanned || workflow.Plan.Recommendation.TestType != item.kind {
				t.Fatalf("plan = %+v", workflow)
			}
			path := "/api/v1/tests/" + workflow.ID
			response = doJSON(t, app, http.MethodPost, path+"/start", nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("start status %d: %s", response.StatusCode, readBody(t, response))
			}
			decodeBody(t, response, &workflow)
			if workflow.Baseline == nil || workflow.Baseline.ID == 0 {
				t.Fatal("before window missing")
			}
			if item.kind == domain.TestMovementCorrelation && workflow.Status != domain.TestWaitingForUser {
				t.Fatalf("movement state = %s", workflow.Status)
			}
			response = doJSON(t, app, http.MethodPost, path+"/capture", nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("capture status %d: %s", response.StatusCode, readBody(t, response))
			}
			decodeBody(t, response, &workflow)
			if workflow.During == nil || workflow.Result == nil || workflow.During.ID == workflow.Baseline.ID {
				t.Fatalf("test capture = %+v", workflow)
			}
			if item.scenario == simulator.ScenarioIntermittent && workflow.Result.Result != "POSITIVE_CORRELATION" {
				t.Fatalf("movement result = %s", workflow.Result.Result)
			}
			if item.scenario == simulator.ScenarioSimultaneousDropouts && workflow.Result.Result != "SHARED_FAILURE_PATTERN" {
				t.Fatalf("shared result = %s", workflow.Result.Result)
			}
			response = doJSON(t, app, http.MethodPost, path+"/remeasure", nil)
			if response.StatusCode != http.StatusOK {
				t.Fatalf("remeasure status %d: %s", response.StatusCode, readBody(t, response))
			}
			decodeBody(t, response, &workflow)
			if workflow.After == nil || workflow.Verification == nil || workflow.After.ID == workflow.During.ID {
				t.Fatalf("verification capture = %+v", workflow)
			}
			if item.scenario != simulator.ScenarioHealthy && workflow.Verification.Status != "RESOLVED" {
				t.Fatalf("verification = %+v", workflow.Verification)
			}
			if item.scenario == simulator.ScenarioHealthy && workflow.Verification.Status != "UNCHANGED" {
				t.Fatalf("healthy verification = %+v", workflow.Verification)
			}
			stored, err := repository.GetTestWorkflow(workflow.ID)
			if err != nil || stored == nil || stored.Verification == nil {
				t.Fatalf("stored workflow = %+v, err = %v", stored, err)
			}
		})
	}
}

func TestGuidedWorkflowRecoversAfterRestartAndPatchRemainsLocked(t *testing.T) {
	app, repository, databasePath, _ := testAppWithVision(t, vision.SkippedAnalyzer{})
	response := doJSON(t, app, http.MethodGet, "/api/v1/tests/recommendation", nil)
	var recommended domain.TestRecommendation
	decodeBody(t, response, &recommended)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests", recommended)
	var workflow domain.DiagnosticWorkflow
	decodeBody(t, response, &workflow)
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/start", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("start: %s", readBody(t, response))
	}
	decodeBody(t, response, &workflow)
	if workflow.Status != domain.TestWaitingForUser {
		t.Fatalf("status = %s", workflow.Status)
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
	response = doJSON(t, restarted, http.MethodGet, "/api/v1/tests/current", nil)
	var recovered domain.DiagnosticWorkflow
	decodeBody(t, response, &recovered)
	if recovered.ID != workflow.ID || recovered.Baseline == nil || recovered.Status != domain.TestWaitingForUser {
		t.Fatalf("recovered = %+v", recovered)
	}
	response = doJSON(t, restarted, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/capture", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("capture after restart: %s", readBody(t, response))
	}
	decodeBody(t, response, &recovered)
	if recovered.Result == nil || recovered.Result.Result != "POSITIVE_CORRELATION" {
		t.Fatalf("recovered result = %+v", recovered.Result)
	}
	recommended.RequiresPatch = true
	response = doJSON(t, restarted, http.MethodPost, "/api/v1/tests", recommended)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("patch plan: %s", readBody(t, response))
	}
	decodeBody(t, response, &workflow)
	if workflow.Status != domain.TestLocked || !strings.Contains(workflow.Plan.Unavailable, "PATCH") {
		t.Fatalf("patch workflow = %+v", workflow)
	}
	response = doJSON(t, restarted, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/start", nil)
	if response.StatusCode != http.StatusLocked {
		t.Fatalf("patch start status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
	response = doJSON(t, restarted, http.MethodPost, "/api/v1/patch", nil)
	if response.StatusCode != http.StatusLocked {
		t.Fatalf("patch endpoint status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
}

func TestGuidedTestRejectsInvalidTransitionsAndChangedProfile(t *testing.T) {
	app, repository := testApp(t)
	response := doJSON(t, app, http.MethodGet, "/api/v1/tests/recommendation", nil)
	var recommendation domain.TestRecommendation
	decodeBody(t, response, &recommendation)
	recommendation.TargetProbes = []string{"P6"}
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests", recommendation)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unassigned target status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
	recommendation.TargetProbes = []string{"P3"}
	response = doJSON(t, app, http.MethodPost, "/api/v1/tests", recommendation)
	var workflow domain.DiagnosticWorkflow
	decodeBody(t, response, &workflow)
	path := "/api/v1/tests/" + workflow.ID
	response = doJSON(t, app, http.MethodPost, path+"/capture", nil)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("capture before start status = %d", response.StatusCode)
	}
	_ = readBody(t, response)
	profile, err := repository.GetProfile(workflow.ProfileID)
	if err != nil || profile == nil {
		t.Fatal(err)
	}
	profile.Version++
	if err := repository.SaveProfile(*profile); err != nil {
		t.Fatal(err)
	}
	response = doJSON(t, app, http.MethodPost, path+"/start", nil)
	if response.StatusCode != http.StatusConflict || !strings.Contains(readBody(t, response), "PROFILE_CHANGED") {
		t.Fatalf("changed profile status = %d", response.StatusCode)
	}
}
