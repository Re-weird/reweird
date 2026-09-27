package httpapi

import (
	"net/http"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
)

func TestSimulatedKnownGoodStaysSimulatedAndPersists(t *testing.T) {
	app, repository := testApp(t)
	healthy := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": simulator.ScenarioHealthy})
	if healthy.StatusCode != http.StatusOK {
		t.Fatalf("healthy scenario status %d: %s", healthy.StatusCode, readBody(t, healthy))
	}
	var session domain.Session
	decodeBody(t, healthy, &session)
	if session.MeasurementID == 0 {
		t.Fatal("healthy capture was not persisted")
	}
	noConfirmation := doJSON(t, app, http.MethodPost, "/api/v1/profiles/ultrasonic-demo/known-good", map[string]any{"measurement_id": session.MeasurementID})
	if noConfirmation.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing confirmation accepted: %d", noConfirmation.StatusCode)
	}
	response := doJSON(t, app, http.MethodPost, "/api/v1/profiles/ultrasonic-demo/known-good", map[string]any{
		"measurement_id": session.MeasurementID, "confirm_healthy": true, "source": "PHYSICAL", "note": "Demo healthy",
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("save status %d: %s", response.StatusCode, readBody(t, response))
	}
	var record domain.KnownGoodBaseline
	decodeBody(t, response, &record)
	if record.Source != domain.BaselineSimulated || record.MeasurementID != session.MeasurementID || len(record.Probes) != 3 {
		t.Fatalf("incorrect source or capture: %+v", record)
	}
	again := doJSON(t, app, http.MethodPost, "/api/v1/profiles/ultrasonic-demo/known-good", map[string]any{"measurement_id": session.MeasurementID, "confirm_healthy": true})
	if again.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate status %d", again.StatusCode)
	}
	workflow := domain.DiagnosticWorkflow{
		ID: "test-11111111111111111111111111111111", ProfileID: "ultrasonic-demo", ProjectID: "ultrasonic-demo", ProfileVersion: 1,
		Status: domain.TestResolved, CreatedAtMS: 1200, UpdatedAtMS: 2400,
		Plan:         domain.TestPlan{Recommendation: domain.TestRecommendation{Reason: "ECHO intermittent"}},
		During:       &domain.MeasurementWindow{ID: 2, ProfileID: "ultrasonic-demo", Source: "simulator", DeviceID: "simulated-box"},
		After:        &domain.MeasurementWindow{ID: 3, ProfileID: "ultrasonic-demo", Source: "simulator", DeviceID: "simulated-box"},
		UserActions:  []domain.UserAction{{ID: "action-1", Description: "User reported replacing the jumper", TimestampMS: 1800}},
		Verification: &domain.VerificationResult{Status: "RESOLVED", Summary: "Subsequent capture matched baseline", BeforeWindowID: 2, AfterWindowID: 3, TimestampMS: 2400},
	}
	if err := repository.SaveTestWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	passportResponse := doJSON(t, app, http.MethodGet, "/api/v1/profiles/ultrasonic-demo/passport", nil)
	var device domain.DevicePassport
	decodeBody(t, passportResponse, &device)
	if device.Status != domain.PassportSimulatedBaseline || len(device.KnownGood) != 1 || device.KnownGood[0].Source != domain.BaselineSimulated {
		t.Fatalf("passport = %+v", device)
	}
	if device.PhysicalBaseline != nil || device.SimulatedBaseline == nil || device.SimulatedBaseline.MeasurementID != session.MeasurementID {
		t.Fatalf("passport confused physical and simulated baselines: %+v", device)
	}
	if len(device.History) != 1 || len(device.VerifiedRepairs) != 1 || device.VerifiedRepairs[0].WorkflowID != workflow.ID || device.VerifiedRepairs[0].Source != domain.BaselineSimulated {
		t.Fatalf("passport history/verified outcomes = %+v, %+v", device.History, device.VerifiedRepairs)
	}
	fault := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": simulator.ScenarioIntermittent})
	if fault.StatusCode != http.StatusOK {
		t.Fatalf("fault scenario status = %d", fault.StatusCode)
	}
	passportResponse = doJSON(t, app, http.MethodGet, "/api/v1/profiles/ultrasonic-demo/passport", nil)
	decodeBody(t, passportResponse, &device)
	if device.Status != domain.PassportSimulatedDeviation {
		t.Fatalf("simulated fault was not labeled as simulated deviation: %s, %s", device.Status, device.StatusDetail)
	}
	hasPhysical, err := repository.HasPhysicalBaseline("ultrasonic-demo")
	if err != nil || hasPhysical {
		t.Fatalf("simulated capture became physical: %v, %v", hasPhysical, err)
	}
}

func TestPhysicalKnownGoodRequiresConfirmedProbePlanAndStoredSerial(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	profile := profiles.UltrasonicDemo()
	profile.ID = project.ID
	profile.ProjectID = project.ID
	profile.ProjectName = project.Name
	project.ProbePlan = &domain.ProbePlan{ProjectID: project.ID, ProfileID: project.ID, Connected: false}
	if err := repository.SaveProjectProfile(project, profile); err != nil {
		t.Fatal(err)
	}
	healthy := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": simulator.ScenarioHealthy})
	var demoSession domain.Session
	decodeBody(t, healthy, &demoSession)
	window := domain.MeasurementWindow{
		ProfileID: project.ID, Source: "serial", DeviceID: "physical-box-1", Sequence: 1,
		CapturedAtMS: demoSession.RawTelemetry.CapturedAtMS, Raw: demoSession.RawTelemetry, Analysis: demoSession.Analysis,
	}
	window.Raw.ProfileID = project.ID
	window.Raw.DeviceID = window.DeviceID
	window.Analysis.ProfileID = project.ID
	window.Analysis.DeviceID = window.DeviceID
	stored, err := repository.SaveMeasurement(window)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/profiles/" + project.ID + "/known-good"
	body := map[string]any{"measurement_id": stored.ID, "confirm_healthy": true, "source": "SIMULATED"}
	blocked := doJSON(t, app, http.MethodPost, path, body)
	if blocked.StatusCode != http.StatusConflict {
		t.Fatalf("unconfirmed probe plan accepted: %d", blocked.StatusCode)
	}
	project.ProbePlan.Connected = true
	project.ProbePlan.ConnectedAtMS = stored.IngestedAtMS + 1
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	tooEarly := doJSON(t, app, http.MethodPost, path, body)
	if tooEarly.StatusCode != http.StatusConflict {
		t.Fatalf("pre-confirmation capture accepted after connection: %d", tooEarly.StatusCode)
	}
	saveAfterConnection := func(sequence uint64) domain.MeasurementWindow {
		t.Helper()
		next := stored
		next.ID = 0
		next.Sequence = sequence
		next.Raw.Sequence = sequence
		next.IngestedAtMS = project.ProbePlan.ConnectedAtMS + int64(sequence)
		next.CapturedAtMS = 0
		next.Raw.CapturedAtMS = 0
		next.Analysis.CapturedAtMS = 0
		saved, err := repository.SaveMeasurement(next)
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	postConnection := saveAfterConnection(2)
	body["measurement_id"] = postConnection.ID
	single := doJSON(t, app, http.MethodPost, path, body)
	if single.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a single physical window became Known Good: %d", single.StatusCode)
	}
	for sequence := uint64(3); sequence < 2+passport.CalibrationWindows; sequence++ {
		postConnection = saveAfterConnection(sequence)
	}
	body["measurement_id"] = postConnection.ID
	accepted := doJSON(t, app, http.MethodPost, path, body)
	if accepted.StatusCode != http.StatusCreated {
		t.Fatalf("physical save status %d: %s", accepted.StatusCode, readBody(t, accepted))
	}
	var baseline domain.KnownGoodBaseline
	decodeBody(t, accepted, &baseline)
	if baseline.Source != domain.BaselinePhysical || baseline.DeviceID != window.DeviceID {
		t.Fatalf("client changed source: %+v", baseline)
	}
	if baseline.Provenance != domain.ProvenanceRealSerial || baseline.WindowCount != passport.CalibrationWindows || baseline.ProbeMappingHash == "" || baseline.ProfileVersion != profile.Version {
		t.Fatalf("physical baseline lost provenance or binding: %+v", baseline)
	}
	passportResponse := doJSON(t, app, http.MethodGet, "/api/v1/profiles/"+project.ID+"/passport", nil)
	var device domain.DevicePassport
	decodeBody(t, passportResponse, &device)
	if device.Status != domain.PassportNeedsVerification || device.ProbePlan == nil || !device.ProbePlan.Connected {
		t.Fatalf("passport = %+v", device)
	}
	if device.PhysicalBaseline == nil || device.PhysicalBaseline.MeasurementID != postConnection.ID || device.SimulatedBaseline != nil {
		t.Fatalf("passport did not expose physical source separately: %+v", device)
	}
	saveAfterConnection(2 + passport.CalibrationWindows)
	passportResponse = doJSON(t, app, http.MethodGet, "/api/v1/profiles/"+project.ID+"/passport", nil)
	decodeBody(t, passportResponse, &device)
	if device.Status != domain.PassportHealthy {
		t.Fatalf("later matching physical capture status = %s: %s", device.Status, device.StatusDetail)
	}
}

func TestProjectSimulatedKnownGoodDoesNotClaimProbeConnection(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	profile := profiles.UltrasonicDemo()
	profile.ID = project.ID
	profile.ProjectID = project.ID
	profile.ProjectName = project.Name
	project.ProbePlan = &domain.ProbePlan{ProjectID: project.ID, ProfileID: project.ID, Connected: false}
	if err := repository.SaveProjectProfile(project, profile); err != nil {
		t.Fatal(err)
	}
	healthy := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": simulator.ScenarioHealthy})
	var session domain.Session
	decodeBody(t, healthy, &session)
	window := domain.MeasurementWindow{
		ProfileID: project.ID, Source: "simulator", DeviceID: session.RawTelemetry.DeviceID, Sequence: session.RawTelemetry.Sequence,
		CapturedAtMS: session.RawTelemetry.CapturedAtMS, Raw: session.RawTelemetry, Analysis: session.Analysis,
	}
	window.Raw.ProfileID = project.ID
	window.Sequence += 9000
	window.Raw.Sequence = window.Sequence
	window.Analysis.ProfileID = project.ID
	stored, err := repository.SaveMeasurement(window)
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, app, http.MethodPost, "/api/v1/profiles/"+project.ID+"/known-good", map[string]any{"measurement_id": stored.ID, "confirm_healthy": true})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("simulated baseline without physical connection status %d: %s", response.StatusCode, readBody(t, response))
	}
	var baseline domain.KnownGoodBaseline
	decodeBody(t, response, &baseline)
	if baseline.Source != domain.BaselineSimulated {
		t.Fatalf("simulation promoted to physical evidence: %+v", baseline)
	}
}
