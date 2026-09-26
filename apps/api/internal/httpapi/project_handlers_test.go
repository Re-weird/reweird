package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projects"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/vision"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

func TestProjectUploadAnalyzeCorrectConfirmAndProbePlan(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)

	codeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/code", map[string]any{
		"filename":  "distance.ino",
		"code_text": "#define TRIG 5\n#define ECHO 18\nvoid setup(){pinMode(TRIG, OUTPUT);pinMode(ECHO, INPUT);}\nlong duration=pulseIn(ECHO,HIGH);",
	})
	if codeResponse.StatusCode != http.StatusOK {
		t.Fatalf("code status = %d body=%s", codeResponse.StatusCode, readBody(t, codeResponse))
	}

	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/analyze", nil)
	if analyzeResponse.StatusCode != http.StatusOK {
		t.Fatalf("analyze status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}
	var analyzed struct {
		Profile  domain.ProjectProfile  `json:"profile"`
		Analysis domain.ProjectAnalysis `json:"analysis"`
	}
	decodeBody(t, analyzeResponse, &analyzed)
	if len(analyzed.Analysis.Code.Pins) != 2 || len(analyzed.Profile.Connections) != 2 || analyzed.Profile.Confirmed {
		t.Fatalf("analysis = %#v profile = %#v", analyzed.Analysis, analyzed.Profile)
	}
	beforePlan := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/probe-plan", nil)
	if beforePlan.StatusCode != http.StatusConflict {
		t.Fatalf("draft probe plan status = %d body=%s", beforePlan.StatusCode, readBody(t, beforePlan))
	}

	analyzed.Profile.ExpectedBehavior = "Measure distance and warn below 20 cm."
	analyzed.Profile.Confirmed = true
	analyzed.Profile.ConfirmedAtMS = 12345
	analyzed.Profile.ConfirmedBy = "attacker"
	analyzed.Profile.AnalysisStatus = "CONFIRMED"
	analyzed.Profile.Probes = profiles.UltrasonicDemo().Probes
	analyzed.Profile.Components[0].Confirmed = true
	analyzed.Profile.Connections[0].Confirmed = true
	correctedResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", analyzed.Profile)
	if correctedResponse.StatusCode != http.StatusOK {
		t.Fatalf("correct status = %d body=%s", correctedResponse.StatusCode, readBody(t, correctedResponse))
	}
	var corrected domain.ProjectProfile
	decodeBody(t, correctedResponse, &corrected)
	if corrected.ExpectedBehavior != "Measure distance and warn below 20 cm." {
		t.Fatalf("correction was not persisted: %#v", corrected)
	}
	if corrected.Confirmed || corrected.ConfirmedAtMS != 0 || corrected.ConfirmedBy != "" || len(corrected.Probes) != 0 || corrected.Components[0].Confirmed || corrected.Connections[0].Confirmed {
		t.Fatalf("client-controlled confirmation fields survived draft correction: %#v", corrected)
	}

	confirmResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/profile/confirm", nil)
	if confirmResponse.StatusCode != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", confirmResponse.StatusCode, readBody(t, confirmResponse))
	}
	var confirmation struct {
		Profile   domain.ProjectProfile `json:"profile"`
		ProbePlan domain.ProbePlan      `json:"probe_plan"`
	}
	decodeBody(t, confirmResponse, &confirmation)
	if !confirmation.Profile.Confirmed || confirmation.Profile.ConfirmedBy != "user" || len(confirmation.ProbePlan.Instructions) != 3 {
		t.Fatalf("confirmation = %#v", confirmation)
	}
	afterPlan := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/probe-plan", nil)
	if afterPlan.StatusCode != http.StatusOK {
		t.Fatalf("confirmed probe plan status = %d body=%s", afterPlan.StatusCode, readBody(t, afterPlan))
	}

	connectedResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/probe-plan/confirm", nil)
	if connectedResponse.StatusCode != http.StatusOK {
		t.Fatalf("connect status = %d body=%s", connectedResponse.StatusCode, readBody(t, connectedResponse))
	}
	stored, err := repository.GetProject(project.ID)
	if err != nil || stored == nil || stored.ProbePlan == nil || !stored.ProbePlan.Connected {
		t.Fatalf("stored project = %#v err=%v", stored, err)
	}
}

func TestGenericProfileRoutesCannotManufactureOrOverwriteConfirmation(t *testing.T) {
	app, repository := testApp(t)
	forged := profiles.UltrasonicDemo()
	forged.ID = "forged-profile"
	forged.ProjectID = ""
	forged.ConfirmedBy = "attacker"
	response := doJSON(t, app, http.MethodPost, "/api/v1/profiles", forged)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("forged POST status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	stored, err := repository.GetProfile(forged.ID)
	if err != nil || stored != nil {
		t.Fatalf("forged profile stored = %#v err=%v", stored, err)
	}

	forged.Confirmed = false
	forged.ConfirmedAtMS = 0
	forged.ConfirmedBy = ""
	forged.AnalysisStatus = "DRAFT"
	forged.Probes = nil
	for index := range forged.Components {
		forged.Components[index].Confirmed = false
	}
	for index := range forged.Connections {
		forged.Connections[index].Confirmed = false
	}
	response = doJSON(t, app, http.MethodPost, "/api/v1/profiles", forged)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("draft POST status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var draft domain.ProjectProfile
	decodeBody(t, response, &draft)
	if draft.Confirmed || draft.ConfirmedAtMS != 0 || draft.ConfirmedBy != "" || len(draft.Probes) != 0 {
		t.Fatalf("draft has trusted state: %#v", draft)
	}

	forged.Confirmed = true
	forged.ConfirmedAtMS = 12345
	forged.ConfirmedBy = "attacker"
	forged.Probes = profiles.UltrasonicDemo().Probes
	response = doJSON(t, app, http.MethodPut, "/api/v1/profiles/"+forged.ID, forged)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("forged PUT status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	stored, err = repository.GetProfile(forged.ID)
	if err != nil || stored == nil || stored.Confirmed {
		t.Fatalf("forged PUT changed draft = %#v err=%v", stored, err)
	}

	confirmed := profiles.UltrasonicDemo()
	confirmed.Confirmed = false
	confirmed.ConfirmedBy = ""
	confirmed.ConfirmedAtMS = 0
	confirmed.Probes = nil
	response = doJSON(t, app, http.MethodPut, "/api/v1/profiles/ultrasonic-demo", confirmed)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("confirmed overwrite status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	stored, err = repository.GetProfile("ultrasonic-demo")
	if err != nil || stored == nil || !stored.Confirmed || len(stored.Probes) == 0 {
		t.Fatalf("confirmed profile was downgraded: %#v err=%v", stored, err)
	}
}

func TestProjectProfileCannotBeChangedOutsideDraftWorkflow(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	code := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/code", map[string]any{
		"filename": "distance.ino", "code_text": "#define TRIG 5\n#define ECHO 18\nvoid setup(){pinMode(TRIG, OUTPUT);pinMode(ECHO, INPUT);}",
	})
	if code.StatusCode != http.StatusOK {
		t.Fatalf("code status = %d body=%s", code.StatusCode, readBody(t, code))
	}
	analysis := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/analyze", nil)
	if analysis.StatusCode != http.StatusOK {
		t.Fatalf("analysis status = %d body=%s", analysis.StatusCode, readBody(t, analysis))
	}
	var analyzed struct {
		Profile domain.ProjectProfile `json:"profile"`
	}
	decodeBody(t, analysis, &analyzed)
	forged := analyzed.Profile
	forged.Conflicts = nil
	response := doJSON(t, app, http.MethodPut, "/api/v1/profiles/"+project.ID, forged)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("generic project-profile PUT status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	confirm := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/profile/confirm", nil)
	if confirm.StatusCode != http.StatusOK {
		t.Fatalf("confirmation status = %d body=%s", confirm.StatusCode, readBody(t, confirm))
	}
	for _, path := range []string{"/code", "/analyze"} {
		response = doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+path, nil)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("confirmed project %s status = %d body=%s", path, response.StatusCode, readBody(t, response))
		}
	}
	stored, err := repository.GetProfile(project.ID)
	if err != nil || stored == nil || !stored.Confirmed {
		t.Fatalf("confirmed project was downgraded: %#v err=%v", stored, err)
	}
}

func TestImageMetadataAndInvalidCodeUpload(t *testing.T) {
	app, _, _, uploadRoot := testAppWithVision(t, vision.SkippedAnalyzer{})
	project := createTestProject(t, app)
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 504)...)
	imageResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/media", "file", "board.png", png)
	if imageResponse.StatusCode != http.StatusOK {
		t.Fatalf("image status = %d body=%s", imageResponse.StatusCode, readBody(t, imageResponse))
	}
	var withImage domain.Project
	decodeBody(t, imageResponse, &withImage)
	if withImage.Image == nil || withImage.Image.ContentType != "image/png" || withImage.Image.StorageRef == "" {
		t.Fatalf("image metadata = %#v", withImage.Image)
	}
	replacement := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1}, make([]byte, 503)...)
	replaceResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/media", "file", "replacement.png", replacement)
	if replaceResponse.StatusCode != http.StatusOK {
		t.Fatalf("replace image status = %d body=%s", replaceResponse.StatusCode, readBody(t, replaceResponse))
	}
	var replaced domain.Project
	decodeBody(t, replaceResponse, &replaced)
	entries, err := os.ReadDir(filepath.Join(uploadRoot, project.ID))
	if err != nil || len(entries) != 1 || replaced.Image == nil || replaced.Image.StorageRef == withImage.Image.StorageRef {
		t.Fatalf("image cleanup: entries=%v project=%#v err=%v", entries, replaced, err)
	}
	oversized := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, projects.MaxImageBytes)...)
	largeResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/media", "file", "large.png", oversized)
	if largeResponse.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized image status = %d body=%s", largeResponse.StatusCode, readBody(t, largeResponse))
	}
	badResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/code", "file", "firmware.bin", []byte{0, 1, 2})
	if badResponse.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("binary code status = %d body=%s", badResponse.StatusCode, readBody(t, badResponse))
	}
	largeCode := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/code", map[string]any{"filename": "large.ino", "code_text": strings.Repeat("x", projects.MaxCodeBytes+1)})
	if largeCode.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized code status = %d body=%s", largeCode.StatusCode, readBody(t, largeCode))
	}
}

func TestUnresolvedConflictBlocksConfirmation(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)
	profile := domain.ProjectProfile{
		ID: project.ID, ProjectID: project.ID, Version: 1, ProjectName: project.Name, Controller: project.Controller, LogicVoltage: project.LogicVoltage,
		Components:  []domain.ComponentSpecification{{ID: "hc-sr04", Name: "HC-SR04", Sources: []domain.ProjectFactSource{domain.SourceVisionAI}, Confidence: .9}},
		Connections: []domain.ProfileConnection{{ID: "echo", ComponentID: "hc-sr04", ComponentName: "HC-SR04", Role: "ECHO", GPIO: pointer(18), Target: "ESP32 GPIO18 / HC-SR04 ECHO", Direction: "input", Behavior: "pulse_input", Expected: domain.ExpectedSignal{SignalType: "pulse input", Required: true}, Sources: []domain.ProjectFactSource{domain.SourceCodeStaticAnalysis}, Required: true}},
		Conflicts:   []domain.ProfileConflict{{ID: "echo-gpio", ConnectionID: "echo", Field: "HC-SR04 ECHO", Options: []domain.ConflictOption{{Value: "GPIO18", Source: domain.SourceCodeStaticAnalysis, Confidence: 1}, {Value: "GPIO17", Source: domain.SourceVisionAI, Confidence: .7}}, RequiresConfirmation: true}},
	}
	if err := repository.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/profile/confirm", nil)
	if response.StatusCode != http.StatusConflict || !strings.Contains(readBody(t, response), "resolve conflict") {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestFullProjectUnderstandingFlowPersistsAcrossRestart(t *testing.T) {
	visionCalled := false
	geminiServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		visionCalled = true
		if request.Header.Get("x-goog-api-key") != "configured-key" {
			t.Fatal("Gemini adapter did not send the configured API key")
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"components\":[{\"catalog_id\":\"hc-sr04\",\"name\":\"HC-SR04\",\"confidence\":0.91,\"visible_labels\":[\"HC-SR04\"]}],\"relationships\":[{\"from\":\"ESP32\",\"to\":\"HC-SR04\",\"role\":\"ECHO\",\"gpio\":17,\"confidence\":0.74}],\"warnings\":[]}"}]}}]}`))
	}))
	defer geminiServer.Close()
	geminiAnalyzer := vision.NewGeminiForTest("configured-key", "fake-gemini", geminiServer.URL, geminiServer.Client())
	app, repository, databasePath, uploadRoot := testAppWithVision(t, geminiAnalyzer)
	project := createTestProject(t, app)

	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	imageResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/media", "file", "bench.jpg", jpeg)
	if imageResponse.StatusCode != http.StatusOK {
		t.Fatalf("image status = %d body=%s", imageResponse.StatusCode, readBody(t, imageResponse))
	}
	var withImage domain.Project
	decodeBody(t, imageResponse, &withImage)

	code := []byte("#define TRIG 5\n#define ECHO 18\nvoid setup(){pinMode(TRIG, OUTPUT);pinMode(ECHO, INPUT);}\nlong duration=pulseIn(ECHO,HIGH);")
	codeResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/code", "file", "distance.ino", code)
	if codeResponse.StatusCode != http.StatusOK {
		t.Fatalf("code status = %d body=%s", codeResponse.StatusCode, readBody(t, codeResponse))
	}

	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/analyze", nil)
	if analyzeResponse.StatusCode != http.StatusOK {
		t.Fatalf("analyze status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}
	var analyzed struct {
		Project  domain.Project         `json:"project"`
		Profile  domain.ProjectProfile  `json:"profile"`
		Analysis domain.ProjectAnalysis `json:"analysis"`
	}
	decodeBody(t, analyzeResponse, &analyzed)
	if analyzed.Project.Image == nil || analyzed.Project.Code == nil || analyzed.Project.Code.Text != string(code) {
		t.Fatalf("stored inputs missing: %#v", analyzed.Project)
	}
	if analyzed.Analysis.Vision.Status != "VISION_COMPLETE" || len(analyzed.Analysis.Code.Pins) != 2 {
		t.Fatalf("analysis = %#v", analyzed.Analysis)
	}
	if !visionCalled {
		t.Fatal("configured Gemini analyzer was not called")
	}
	if len(analyzed.Profile.Conflicts) != 1 || analyzed.Profile.Conflicts[0].Resolved {
		t.Fatalf("conflicts = %#v", analyzed.Profile.Conflicts)
	}
	if len(analyzed.Profile.Components) != 1 || !hasFactSource(analyzed.Profile.Components[0].Sources, domain.SourceCatalog) || !hasFactSource(analyzed.Profile.Components[0].Sources, domain.SourceVisionAI) {
		t.Fatalf("catalog/vision provenance = %#v", analyzed.Profile.Components)
	}

	blocked := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/profile/confirm", nil)
	if blocked.StatusCode != http.StatusConflict {
		t.Fatalf("unresolved confirmation status = %d body=%s", blocked.StatusCode, readBody(t, blocked))
	}

	analyzed.Profile.ExpectedBehavior = "Measure distance and alarm below 20 cm."
	analyzed.Profile.Conflicts[0].Resolution = "GPIO18"
	analyzed.Profile.Conflicts[0].Resolved = true
	correctedResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", analyzed.Profile)
	if correctedResponse.StatusCode != http.StatusOK {
		t.Fatalf("correct status = %d body=%s", correctedResponse.StatusCode, readBody(t, correctedResponse))
	}
	var corrected domain.ProjectProfile
	decodeBody(t, correctedResponse, &corrected)
	if corrected.Version != 2 || corrected.ExpectedBehavior != "Measure distance and alarm below 20 cm." || !corrected.Conflicts[0].Resolved {
		t.Fatalf("corrected profile = %#v", corrected)
	}
	if hasFactSource(corrected.Components[0].Sources, domain.SourceUser) {
		t.Fatalf("unchanged component was incorrectly marked as a user correction: %#v", corrected.Components[0].Sources)
	}
	echo := connectionByRole(corrected.Connections, "ECHO")
	if echo == nil || echo.GPIO == nil || *echo.GPIO != 18 || !hasFactSource(echo.Sources, domain.SourceUser) {
		t.Fatalf("resolved connection = %#v", echo)
	}
	unchangedResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", corrected)
	if unchangedResponse.StatusCode != http.StatusOK {
		t.Fatalf("idempotent save status = %d body=%s", unchangedResponse.StatusCode, readBody(t, unchangedResponse))
	}
	var unchanged domain.ProjectProfile
	decodeBody(t, unchangedResponse, &unchanged)
	if unchanged.Version != corrected.Version {
		t.Fatalf("unchanged save advanced revision from %d to %d", corrected.Version, unchanged.Version)
	}

	confirmResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/profile/confirm", nil)
	if confirmResponse.StatusCode != http.StatusOK {
		t.Fatalf("confirm status = %d body=%s", confirmResponse.StatusCode, readBody(t, confirmResponse))
	}
	var confirmed struct {
		Profile   domain.ProjectProfile `json:"profile"`
		ProbePlan domain.ProbePlan      `json:"probe_plan"`
	}
	decodeBody(t, confirmResponse, &confirmed)
	if !confirmed.Profile.Confirmed || confirmed.Profile.ConfirmedAtMS == 0 || len(confirmed.ProbePlan.Instructions) != 3 {
		t.Fatalf("confirmation = %#v", confirmed)
	}

	connectedResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/probe-plan/confirm", nil)
	if connectedResponse.StatusCode != http.StatusOK {
		t.Fatalf("connect status = %d body=%s", connectedResponse.StatusCode, readBody(t, connectedResponse))
	}
	var connected domain.ProbePlan
	decodeBody(t, connectedResponse, &connected)
	if !connected.Connected || connected.ConnectedAtMS == 0 {
		t.Fatalf("connected plan = %#v", connected)
	}

	// The separate demo must remain usable; its simulator data is not compatible
	// with this project's generated probe modes.
	sessionResponse := doJSON(t, app, http.MethodGet, "/api/v1/session", nil)
	if sessionResponse.StatusCode != http.StatusOK {
		t.Fatalf("demo session status = %d body=%s", sessionResponse.StatusCode, readBody(t, sessionResponse))
	}
	var session domain.Session
	decodeBody(t, sessionResponse, &session)
	if session.ProfileID != "ultrasonic-demo" {
		t.Fatalf("simulator profile = %q, want ultrasonic-demo", session.ProfileID)
	}

	if withImage.Image == nil {
		t.Fatal("image metadata missing")
	}
	if _, err := os.Stat(filepath.Join(uploadRoot, filepath.FromSlash(withImage.Image.StorageRef))); err != nil {
		t.Fatalf("stored image missing: %v", err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewApp(diagnostics.NewEngine(signalanalysis.New()), reopened, simulator.NewUltrasonicSource(), "ultrasonic-demo", ProjectServices{Understanding: projectunderstanding.New(codeanalysis.New(), vision.SkippedAnalyzer{}, catalog), UploadRoot: uploadRoot})
	projectResponse := doJSON(t, restarted, http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	var persistedProject domain.Project
	decodeBody(t, projectResponse, &persistedProject)
	profileResponse := doJSON(t, restarted, http.MethodGet, "/api/v1/projects/"+project.ID+"/profile", nil)
	var persistedProfile domain.ProjectProfile
	decodeBody(t, profileResponse, &persistedProfile)
	if persistedProject.ProbePlan == nil || !persistedProject.ProbePlan.Connected || persistedProject.Analysis == nil || persistedProject.Analysis.Vision.Status != "VISION_COMPLETE" {
		t.Fatalf("project after restart = %#v", persistedProject)
	}
	if !persistedProfile.Confirmed || persistedProfile.Version != 2 || persistedProfile.ExpectedBehavior != corrected.ExpectedBehavior {
		t.Fatalf("profile after restart = %#v", persistedProfile)
	}
}

func testApp(t *testing.T) (*fiber.App, *store.SQLiteStore) {
	t.Helper()
	app, repository, _, _ := testAppWithVision(t, vision.SkippedAnalyzer{})
	return app, repository
}

func TestSimulatorScenarioAPIStoresRawAndDerivedWindow(t *testing.T) {
	app, _ := testApp(t)
	listResponse := doJSON(t, app, http.MethodGet, "/api/v1/simulator/scenarios", nil)
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("scenario list status = %d body=%s", listResponse.StatusCode, readBody(t, listResponse))
	}
	var available struct {
		Active    string                     `json:"active"`
		Scenarios []domain.SimulatorScenario `json:"scenarios"`
	}
	decodeBody(t, listResponse, &available)
	if len(available.Scenarios) != 9 {
		t.Fatalf("scenarios = %d, want 9", len(available.Scenarios))
	}

	runResponse := doJSON(t, app, http.MethodPost, "/api/v1/simulator/scenario", map[string]any{"scenario_id": simulator.ScenarioTimingDrift})
	if runResponse.StatusCode != http.StatusOK {
		t.Fatalf("run status = %d body=%s", runResponse.StatusCode, readBody(t, runResponse))
	}
	var session domain.Session
	decodeBody(t, runResponse, &session)
	if session.ScenarioID != simulator.ScenarioTimingDrift || session.MeasurementID == 0 || session.RawTelemetry.ProfileID != "ultrasonic-demo" {
		t.Fatalf("session = %#v", session)
	}
	if !strings.Contains(session.Diagnosis.Headline, "timing outside specification") {
		t.Fatalf("diagnosis = %#v", session.Diagnosis)
	}

	measurementsResponse := doJSON(t, app, http.MethodGet, "/api/v1/measurements?profile_id=ultrasonic-demo&limit=10", nil)
	if measurementsResponse.StatusCode != http.StatusOK {
		t.Fatalf("measurements status = %d body=%s", measurementsResponse.StatusCode, readBody(t, measurementsResponse))
	}
	var windows []domain.MeasurementWindow
	decodeBody(t, measurementsResponse, &windows)
	if len(windows) == 0 || windows[0].Raw.ProfileID != windows[0].Analysis.ProfileID {
		t.Fatalf("measurements = %#v", windows)
	}
}

func testAppWithVision(t *testing.T, visionAnalyzer vision.Analyzer) (*fiber.App, *store.SQLiteStore, string, string) {
	t.Helper()
	root := t.TempDir()
	databasePath := filepath.Join(root, "test.db")
	uploadRoot := filepath.Join(root, "uploads")
	repository, err := store.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	demo := profiles.UltrasonicDemo()
	if err := repository.SaveProfile(demo); err != nil {
		t.Fatal(err)
	}
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	understanding := projectunderstanding.New(codeanalysis.New(), visionAnalyzer, catalog)
	return NewApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, ProjectServices{Understanding: understanding, UploadRoot: uploadRoot}), repository, databasePath, uploadRoot
}

func createTestProject(t *testing.T, app *fiber.App) domain.Project {
	t.Helper()
	response := doJSON(t, app, http.MethodPost, "/api/v1/projects", map[string]any{"name": "Distance Alarm", "description": "Measure distance", "controller": "ESP32", "logic_voltage": 3.3})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var project domain.Project
	decodeBody(t, response, &project)
	return project
}

func doJSON(t *testing.T, app *fiber.App, method, path string, payload any) *http.Response {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func doMultipart(t *testing.T, app *fiber.App, path, field, filename string, payload []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeBody(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return string(payload)
}

func pointer(value int) *int { return &value }

func hasFactSource(sources []domain.ProjectFactSource, expected domain.ProjectFactSource) bool {
	for _, source := range sources {
		if source == expected {
			return true
		}
	}
	return false
}

func connectionByRole(connections []domain.ProfileConnection, role string) *domain.ProfileConnection {
	for index := range connections {
		if connections[index].Role == role {
			return &connections[index]
		}
	}
	return nil
}

func TestProjectVisibilityDefaultsPrivateAndPersists(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	if project.Visibility != domain.VisibilityPrivate {
		t.Fatalf("new project visibility = %q, want private", project.Visibility)
	}

	response := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/visibility", map[string]any{"visibility": "public"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("visibility status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var updated domain.Project
	decodeBody(t, response, &updated)
	if updated.Visibility != domain.VisibilityPublic {
		t.Fatalf("updated visibility = %q, want public", updated.Visibility)
	}

	listResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects", nil)
	var listed []domain.Project
	decodeBody(t, listResponse, &listed)
	if len(listed) != 1 || listed[0].Visibility != domain.VisibilityPublic {
		t.Fatalf("listed = %#v", listed)
	}

	invalid := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/visibility", map[string]any{"visibility": "secret"})
	if invalid.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid visibility status = %d, want 422", invalid.StatusCode)
	}
	missing := doJSON(t, app, http.MethodPut, "/api/v1/projects/nope/visibility", map[string]any{"visibility": "public"})
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing project status = %d, want 404", missing.StatusCode)
	}
}
