package httpapi

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
)

// doPhysicalCommitMultipartAs mirrors doPhysicalCommitMultipart but sets the
// X-Test-Owner header used by testOwnedApp's fake owner-identity middleware.
func doPhysicalCommitMultipartAs(t *testing.T, app *fiber.App, path, owner, filename string, image []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if owner != "" {
		request.Header.Set("X-Test-Owner", owner)
	}
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

// countingVisionAnalyzer is a local test double for vision.Analyzer (the
// same seam vision.SkippedAnalyzer/GeminiAnalyzer already implement),
// letting these tests control the Gemini result deterministically and
// count invocations without ever hitting the network.
type countingVisionAnalyzer struct {
	calls  int
	result domain.VisionAnalysis
}

func (analyzer *countingVisionAnalyzer) Analyze(context.Context, string, string) domain.VisionAnalysis {
	analyzer.calls++
	return analyzer.result
}

func TestAnalyzePhysicalCommitHardwareWithValidImage(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE", Model: "test-model", Components: []domain.VisionComponent{
		{CatalogID: "hc-sr04", Name: "HC-SR04", Confidence: 0.94, Source: domain.SourceVisionAI},
	}}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if analyzeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("analyze status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}
	var stored domain.PhysicalCommitVisionAnalysis
	decodeBody(t, analyzeResponse, &stored)
	if stored.PhysicalCommitID != commit.ID || stored.ProjectID != project.ID || stored.Provider != "gemini" {
		t.Fatalf("stored = %#v", stored)
	}
	if stored.Analysis.Status != "VISION_COMPLETE" || len(stored.Analysis.Components) != 1 || stored.Analysis.Components[0].Name != "HC-SR04" {
		t.Fatalf("analysis = %#v", stored.Analysis)
	}
	if analyzer.calls != 1 {
		t.Fatalf("expected exactly one Gemini call, got %d", analyzer.calls)
	}
}

func TestRetrievePreviouslyStoredVisionAnalysis(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "SG90 Servo", Confidence: 0.8, Source: domain.SourceVisionAI}}}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if analyzeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("analyze status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}

	// GET must never invoke Gemini -- assert the call counter does not move.
	callsBeforeGet := analyzer.calls
	getResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/vision-analysis", nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResponse.StatusCode, readBody(t, getResponse))
	}
	if analyzer.calls != callsBeforeGet {
		t.Fatalf("GET invoked the vision analyzer: calls went from %d to %d", callsBeforeGet, analyzer.calls)
	}
	var stored domain.PhysicalCommitVisionAnalysis
	decodeBody(t, getResponse, &stored)
	if len(stored.Analysis.Components) != 1 || stored.Analysis.Components[0].Name != "SG90 Servo" {
		t.Fatalf("stored = %#v", stored.Analysis)
	}

	// A second GET returns the same persisted record, still without calling Gemini.
	secondGet := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/vision-analysis", nil)
	if secondGet.StatusCode != http.StatusOK {
		t.Fatalf("second get status = %d", secondGet.StatusCode)
	}
	if analyzer.calls != callsBeforeGet {
		t.Fatalf("repeated GET invoked the vision analyzer: calls = %d", analyzer.calls)
	}
}

func TestAnalyzeHardwareRejectsCommitWithNoImage(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE"}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if analyzeResponse.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}
	if analyzer.calls != 0 {
		t.Fatalf("Gemini must never be called for a commit with no image, calls = %d", analyzer.calls)
	}

	getResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/vision-analysis", nil)
	if getResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("get status = %d body=%s (expected VISION_ANALYSIS_NOT_FOUND)", getResponse.StatusCode, readBody(t, getResponse))
	}
}

func TestAnalyzeHardwareMalformedAndMissingCommitID(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	malformed := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/not-a-real-id/analyze-hardware", nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed status = %d body=%s", malformed.StatusCode, readBody(t, malformed))
	}
	missing := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/pcommit-00000000000000000000000000000000/analyze-hardware", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d body=%s", missing.StatusCode, readBody(t, missing))
	}

	malformedGet := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/not-a-real-id/vision-analysis", nil)
	if malformedGet.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed get status = %d body=%s", malformedGet.StatusCode, readBody(t, malformedGet))
	}
}

func TestAnalyzeHardwareCrossProjectIsolation(t *testing.T) {
	app := testOwnedApp(t)
	createA := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Project A", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	var projectA domain.Project
	decodeBody(t, createA, &projectA)
	createB := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-b", map[string]any{"name": "Project B", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	var projectB domain.Project
	decodeBody(t, createB, &projectB)

	commitA := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-a", nil)
	var a domain.PhysicalCommit
	decodeBody(t, commitA, &a)

	// B cannot analyze or read A's commit through B's own project id.
	crossAnalyze := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectB.ID+"/physical-commits/"+a.ID+"/analyze-hardware", "owner-b", nil)
	if crossAnalyze.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project analyze status = %d body=%s", crossAnalyze.StatusCode, readBody(t, crossAnalyze))
	}
	crossGet := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectB.ID+"/physical-commits/"+a.ID+"/vision-analysis", "owner-b", nil)
	if crossGet.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project get status = %d body=%s", crossGet.StatusCode, readBody(t, crossGet))
	}
}

// TestAnalyzeHardwareFailureNeverPersistsOrMutates: a VISION_FAILED result
// must not be stored and must not touch the commit's image or profile
// snapshot, and must not overwrite a prior successful analysis.
func TestAnalyzeHardwareFailureNeverPersistsOrMutates(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "HC-SR04", Confidence: 0.9, Source: domain.SourceVisionAI}}}}
	app, repository, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	firstAnalyze := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if firstAnalyze.StatusCode != http.StatusCreated {
		t.Fatalf("first analyze status = %d body=%s", firstAnalyze.StatusCode, readBody(t, firstAnalyze))
	}

	// Now make Gemini "fail" and re-analyze.
	analyzer.result = domain.VisionAnalysis{Status: "VISION_FAILED", Warnings: []string{"Gemini request failed: simulated timeout"}}
	failedAnalyze := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if failedAnalyze.StatusCode != http.StatusBadGateway {
		t.Fatalf("failed analyze status = %d body=%s", failedAnalyze.StatusCode, readBody(t, failedAnalyze))
	}

	// The commit itself, its image, and profile snapshot must be untouched.
	refetchedCommit, err := repository.GetPhysicalCommit(project.ID, commit.ID)
	if err != nil || refetchedCommit == nil || refetchedCommit.Image == nil || refetchedCommit.Image.SHA256 != commit.Image.SHA256 {
		t.Fatalf("commit image changed after a failed analysis: %#v, err=%v", refetchedCommit, err)
	}

	// The prior successful analysis must still be the one on record.
	getResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/vision-analysis", nil)
	if getResponse.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getResponse.StatusCode, readBody(t, getResponse))
	}
	var stored domain.PhysicalCommitVisionAnalysis
	decodeBody(t, getResponse, &stored)
	if stored.Analysis.Status != "VISION_COMPLETE" || len(stored.Analysis.Components) != 1 || stored.Analysis.Components[0].Name != "HC-SR04" {
		t.Fatalf("a failed re-analysis overwrote the prior successful one: %#v", stored.Analysis)
	}
}

// TestAnalyzeHardwareSkippedNeverPersistsOrOverwrites: Gemini not being
// configured (VISION_SKIPPED) is neither evidence nor interpretation -- it
// must be rejected with 503, never persisted, and never overwrite a prior
// successful analysis.
func TestAnalyzeHardwareSkippedNeverPersistsOrOverwrites(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_SKIPPED", Warnings: []string{"Gemini Vision was skipped because GEMINI_API_KEY is not configured."}}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if analyzeResponse.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}

	// The commit remains "not analyzed" -- nothing was persisted.
	getResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/vision-analysis", nil)
	if getResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("get status = %d body=%s (expected VISION_ANALYSIS_NOT_FOUND)", getResponse.StatusCode, readBody(t, getResponse))
	}
}

func TestAnalyzeHardwareUnavailableWhenUnderstandingNotConfigured(t *testing.T) {
	app := testOwnedApp(t) // ProjectServices{} -- no Understanding configured
	created := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Project A", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	var project domain.Project
	decodeBody(t, created, &project)

	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipartAs(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "owner-a", "bench.jpg", jpeg)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	if commit.Image == nil {
		t.Fatalf("expected the commit to have a captured image: %#v", commit)
	}

	analyzeResponse := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", "owner-a", nil)
	if analyzeResponse.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s (expected ANALYSIS_UNAVAILABLE)", analyzeResponse.StatusCode, readBody(t, analyzeResponse))
	}
}
