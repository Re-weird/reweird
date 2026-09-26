package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
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

	analyzed.Profile.ExpectedBehavior = "Measure distance and warn below 20 cm."
	correctedResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", analyzed.Profile)
	if correctedResponse.StatusCode != http.StatusOK {
		t.Fatalf("correct status = %d body=%s", correctedResponse.StatusCode, readBody(t, correctedResponse))
	}
	var corrected domain.ProjectProfile
	decodeBody(t, correctedResponse, &corrected)
	if corrected.ExpectedBehavior != "Measure distance and warn below 20 cm." {
		t.Fatalf("correction was not persisted: %#v", corrected)
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

	connectedResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/probe-plan/confirm", nil)
	if connectedResponse.StatusCode != http.StatusOK {
		t.Fatalf("connect status = %d body=%s", connectedResponse.StatusCode, readBody(t, connectedResponse))
	}
	stored, err := repository.GetProject(project.ID)
	if err != nil || stored == nil || stored.ProbePlan == nil || !stored.ProbePlan.Connected {
		t.Fatalf("stored project = %#v err=%v", stored, err)
	}
}

func TestImageMetadataAndInvalidCodeUpload(t *testing.T) {
	app, _ := testApp(t)
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
	badResponse := doMultipart(t, app, "/api/v1/projects/"+project.ID+"/code", "file", "firmware.bin", []byte{0, 1, 2})
	if badResponse.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("binary code status = %d body=%s", badResponse.StatusCode, readBody(t, badResponse))
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

func testApp(t *testing.T) (*fiber.App, *store.SQLiteStore) {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
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
	understanding := projectunderstanding.New(codeanalysis.New(), vision.SkippedAnalyzer{}, catalog)
	return NewApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, ProjectServices{Understanding: understanding, UploadRoot: filepath.Join(t.TempDir(), "uploads")}), repository
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
