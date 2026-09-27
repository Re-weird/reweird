package httpapi

import (
	"net/http"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestRestoreWithNoOtherCommitIsUnavailable(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	restoreResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", restoreResponse.StatusCode, readBody(t, restoreResponse))
	}
	var plan domain.PhysicalRestorePlan
	decodeBody(t, restoreResponse, &plan)
	if plan.HasSource {
		t.Fatalf("expected no source commit, got %#v", plan)
	}
	if plan.Components.Status != domain.RestoreUnavailable || plan.Circuit.Status != domain.RestoreUnavailable {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestRestoreDefaultsToNewestOtherCommit(t *testing.T) {
	app, _ := testApp(t)
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

	// The first commit is captured only once a ProjectProfile exists, so
	// both commits below have a comparable component snapshot.
	first := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var firstCommit domain.PhysicalCommit
	decodeBody(t, first, &firstCommit)

	var draft domain.ProjectProfile
	profileResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/profile", nil)
	if profileResponse.StatusCode != http.StatusOK {
		t.Fatalf("profile status = %d body=%s", profileResponse.StatusCode, readBody(t, profileResponse))
	}
	decodeBody(t, profileResponse, &draft)
	draft.Components = append(draft.Components, domain.ComponentSpecification{ID: "servo-1", Name: "SG90 Servo"})
	updateResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", draft)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.StatusCode, readBody(t, updateResponse))
	}

	second := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var secondCommit domain.PhysicalCommit
	decodeBody(t, second, &secondCommit)

	// Restoring toward the first (older, without the servo) should default
	// its reference/source to the newest commit -- the second one -- and
	// report that the servo needs to be removed to match the target.
	restoreResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+firstCommit.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", restoreResponse.StatusCode, readBody(t, restoreResponse))
	}
	var plan domain.PhysicalRestorePlan
	decodeBody(t, restoreResponse, &plan)
	if !plan.HasSource || plan.SourceCommit != secondCommit.ID {
		t.Fatalf("expected default source = newest commit %s, got %#v", secondCommit.ID, plan)
	}
	if plan.Components.Status != domain.RestoreActionRequired {
		t.Fatalf("components = %#v", plan.Components)
	}
	found := false
	for _, action := range plan.Components.Actions {
		if action.Title == "SG90 Servo" && action.Status == domain.RestoreActionRequired {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an action for the extra SG90 Servo, got %#v", plan.Components.Actions)
	}
}

func TestRestoreExplicitSourceQueryParameter(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	first := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var firstCommit domain.PhysicalCommit
	decodeBody(t, first, &firstCommit)
	second := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var secondCommit domain.PhysicalCommit
	decodeBody(t, second, &secondCommit)

	restoreResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+firstCommit.ID+"/restore?source="+secondCommit.ID, nil)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", restoreResponse.StatusCode, readBody(t, restoreResponse))
	}
	var plan domain.PhysicalRestorePlan
	decodeBody(t, restoreResponse, &plan)
	if plan.SourceCommit != secondCommit.ID {
		t.Fatalf("plan.SourceCommit = %s, want %s", plan.SourceCommit, secondCommit.ID)
	}
}

func TestRestoreMalformedAndMissingIDs(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	malformed := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/not-real/restore", nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed status = %d", malformed.StatusCode)
	}
	missing := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/pcommit-00000000000000000000000000000000/restore", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.StatusCode)
	}
	malformedSource := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/restore?source=not-real", nil)
	if malformedSource.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed source status = %d", malformedSource.StatusCode)
	}
}

func TestRestoreCrossProjectRejected(t *testing.T) {
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

	cross := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectB.ID+"/physical-commits/"+a.ID+"/restore", "owner-b", nil)
	if cross.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project restore status = %d body=%s", cross.StatusCode, readBody(t, cross))
	}
}

// TestRestoreNeverInvokesGemini proves Restore is computed purely from
// already-persisted evidence, even when both compared commits have a
// stored vision analysis.
func TestRestoreNeverInvokesGemini(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "HC-SR04"}}}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}

	first := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	var firstCommit domain.PhysicalCommit
	decodeBody(t, first, &firstCommit)
	analyzeFirst := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+firstCommit.ID+"/analyze-hardware", nil)
	if analyzeFirst.StatusCode != http.StatusCreated {
		t.Fatalf("analyze first status = %d", analyzeFirst.StatusCode)
	}
	second := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench2.jpg", jpeg)
	var secondCommit domain.PhysicalCommit
	decodeBody(t, second, &secondCommit)
	analyzeSecond := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+secondCommit.ID+"/analyze-hardware", nil)
	if analyzeSecond.StatusCode != http.StatusCreated {
		t.Fatalf("analyze second status = %d", analyzeSecond.StatusCode)
	}

	analyzer.calls = 0
	restoreResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+firstCommit.ID+"/restore", nil)
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", restoreResponse.StatusCode, readBody(t, restoreResponse))
	}
	if analyzer.calls != 0 {
		t.Fatalf("restore invoked Gemini: calls = %d", analyzer.calls)
	}
}

func TestVerifyBasicResponseShape(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	verifyResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/verify", nil)
	if verifyResponse.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verifyResponse.StatusCode, readBody(t, verifyResponse))
	}
	var result domain.PhysicalVerifyResult
	decodeBody(t, verifyResponse, &result)
	if result.TargetCommit != commit.ID {
		t.Fatalf("result = %#v", result)
	}
	if result.Overall == "" {
		t.Fatalf("expected a non-empty overall status, got %#v", result)
	}
}

func TestVerifyMalformedAndMissingIDs(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	malformed := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/not-real/verify", nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed status = %d", malformed.StatusCode)
	}
	missing := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/pcommit-00000000000000000000000000000000/verify", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.StatusCode)
	}
}

func TestVerifyCrossProjectRejected(t *testing.T) {
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

	cross := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectB.ID+"/physical-commits/"+a.ID+"/verify", "owner-b", nil)
	if cross.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project verify status = %d body=%s", cross.StatusCode, readBody(t, cross))
	}
}

func TestVerifyPartialEvidenceNeverFabricatesSupport(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	verifyResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/verify", nil)
	var result domain.PhysicalVerifyResult
	decodeBody(t, verifyResponse, &result)
	if result.Electrical.Status == domain.VerifySupported && commit.MeasurementID == nil {
		t.Fatalf("electrical falsely reported SUPPORTED with no captured measurement: %#v", result.Electrical)
	}
	if result.Visual.Status == domain.VerifySupported && commit.Image == nil {
		t.Fatalf("visual falsely reported SUPPORTED with no captured image: %#v", result.Visual)
	}
}

// TestVerifyNeverInvokesGemini proves Verify is computed purely from
// already-persisted/already-observed evidence.
func TestVerifyNeverInvokesGemini(t *testing.T) {
	analyzer := &countingVisionAnalyzer{result: domain.VisionAnalysis{Status: "VISION_COMPLETE", Components: []domain.VisionComponent{{Name: "HC-SR04"}}}}
	app, _, _, _ := testAppWithVision(t, analyzer)
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}
	commitResponse := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "bench.jpg", jpeg)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	analyzeResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/analyze-hardware", nil)
	if analyzeResponse.StatusCode != http.StatusCreated {
		t.Fatalf("analyze status = %d", analyzeResponse.StatusCode)
	}

	analyzer.calls = 0
	verifyResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/verify", nil)
	if verifyResponse.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d body=%s", verifyResponse.StatusCode, readBody(t, verifyResponse))
	}
	if analyzer.calls != 0 {
		t.Fatalf("verify invoked Gemini: calls = %d", analyzer.calls)
	}
}
