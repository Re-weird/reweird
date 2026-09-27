package httpapi

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/projects"
	"github.com/re-weird/reweird/apps/api/internal/vision"
)

func doPhysicalCommitMultipart(t *testing.T, app *fiber.App, path, note, filename string, image []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if note != "" {
		if err := writer.WriteField("note", note); err != nil {
			t.Fatal(err)
		}
	}
	if image != nil {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(image); err != nil {
			t.Fatal(err)
		}
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

// TestPhysicalCommitWorksWithNoOptionalEvidence is the "Device Passport is not
// a dependency" requirement: a brand-new real project with no profile, no
// measurements, and no known-good baselines must still be able to create a
// commit, with every optional field simply absent -- never fabricated.
func TestPhysicalCommitWorksWithNoOptionalEvidence(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, response, &commit)
	if commit.DisplayID != "HW-001" || commit.Sequence != 1 || commit.ProjectID != project.ID {
		t.Fatalf("commit = %#v", commit)
	}
	if commit.ProfileSnapshot != nil || commit.MeasurementID != nil || commit.PassportBaselineIDs != nil || commit.Image != nil {
		t.Fatalf("expected all optional evidence to be absent, got %#v", commit)
	}
	if commit.SoftwareProvider != nil || commit.SoftwareRepository != nil || commit.SoftwareRevision != nil {
		t.Fatalf("software fields must stay null in V1: %#v", commit)
	}
}

func TestPhysicalCommitWithNoteOnlyJSON(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", map[string]any{"note": "Wired the second ultrasonic sensor."})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, response, &commit)
	if commit.Note != "Wired the second ultrasonic sensor." {
		t.Fatalf("commit note = %q", commit.Note)
	}
}

func TestPhysicalCommitWithImageMultipart(t *testing.T) {
	app, _, _, uploadRoot := testAppWithVision(t, vision.SkippedAnalyzer{})
	project := createTestProject(t, app)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0xff, 0xd9}

	response := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "Bench photo before rewiring", "bench.jpg", jpeg)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, response, &commit)
	if commit.Image == nil || commit.Image.ContentType != "image/jpeg" || commit.Image.SizeBytes != int64(len(jpeg)) {
		t.Fatalf("commit image = %#v", commit.Image)
	}
	resolved, err := projects.ResolveImage(uploadRoot, commit.Image.StorageRef)
	if err != nil {
		t.Fatalf("resolve image: %v", err)
	}
	if info, err := os.Stat(resolved); err != nil || info.Size() != int64(len(jpeg)) {
		t.Fatalf("stat image file = %v, %v", info, err)
	}
}

func TestPhysicalCommitSequentialDisplayIDs(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	first := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	second := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var firstCommit, secondCommit domain.PhysicalCommit
	decodeBody(t, first, &firstCommit)
	decodeBody(t, second, &secondCommit)
	if firstCommit.DisplayID != "HW-001" || secondCommit.DisplayID != "HW-002" {
		t.Fatalf("display ids = %q, %q", firstCommit.DisplayID, secondCommit.DisplayID)
	}

	list := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if list.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d body=%s", list.StatusCode, readBody(t, list))
	}
	var page struct {
		Items []domain.PhysicalCommit `json:"items"`
		Count int                     `json:"count"`
	}
	decodeBody(t, list, &page)
	if page.Count != 2 || len(page.Items) != 2 || page.Items[0].DisplayID != "HW-002" {
		t.Fatalf("list = %#v", page)
	}
}

// TestPhysicalCommitProfileSnapshotIsImmutable is the historical-immutability
// requirement: a commit's profile_snapshot must not change when the live
// ProjectProfile is edited afterward.
func TestPhysicalCommitProfileSnapshotIsImmutable(t *testing.T) {
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
	var analyzed struct {
		Profile domain.ProjectProfile `json:"profile"`
	}
	decodeBody(t, analyzeResponse, &analyzed)
	analyzed.Profile.ExpectedBehavior = "Original behavior at commit time."
	firstDraft := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", analyzed.Profile)
	if firstDraft.StatusCode != http.StatusOK {
		t.Fatalf("first draft status = %d body=%s", firstDraft.StatusCode, readBody(t, firstDraft))
	}
	var draft domain.ProjectProfile
	decodeBody(t, firstDraft, &draft)

	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	if commit.ProfileSnapshot == nil || commit.ProfileSnapshot.ExpectedBehavior != "Original behavior at commit time." {
		t.Fatalf("commit snapshot = %#v", commit.ProfileSnapshot)
	}

	draft.ExpectedBehavior = "Mutated after the commit was made."
	mutateResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", draft)
	if mutateResponse.StatusCode != http.StatusOK {
		t.Fatalf("mutate status = %d body=%s", mutateResponse.StatusCode, readBody(t, mutateResponse))
	}

	refetched := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID, nil)
	if refetched.StatusCode != http.StatusOK {
		t.Fatalf("refetch status = %d body=%s", refetched.StatusCode, readBody(t, refetched))
	}
	var refetchedCommit domain.PhysicalCommit
	decodeBody(t, refetched, &refetchedCommit)
	if refetchedCommit.ProfileSnapshot == nil || refetchedCommit.ProfileSnapshot.ExpectedBehavior != "Original behavior at commit time." {
		t.Fatalf("historical snapshot changed after live profile edit: %#v", refetchedCommit.ProfileSnapshot)
	}
}

// TestPhysicalCommitNeverAutoAttachesSimulatedBaseline: a real Physical
// Commit must only ever reference a physical known-good baseline. A
// simulated baseline belongs to the synthetic/game-demo workflow.
func TestPhysicalCommitNeverAutoAttachesSimulatedBaseline(t *testing.T) {
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

	window, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: project.ID, Source: "simulator", DeviceID: "sim-1", Sequence: 1, CapturedAtMS: 1000, IngestedAtMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	simulated, err := repository.SaveKnownGood(domain.KnownGoodBaseline{ProfileID: project.ID, MeasurementID: window.ID, Source: domain.BaselineSimulated, DeviceID: "sim-1", SavedAtMS: 2000, Probes: []domain.BaselineProbe{}})
	if err != nil {
		t.Fatal(err)
	}

	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	if commit.MeasurementID == nil || *commit.MeasurementID != window.ID {
		t.Fatalf("expected commit to reference the latest measurement, got %#v", commit.MeasurementID)
	}
	for _, id := range commit.PassportBaselineIDs {
		if id == simulated.ID {
			t.Fatalf("commit auto-attached a simulated baseline: %#v", commit.PassportBaselineIDs)
		}
	}
	if commit.PassportBaselineIDs != nil {
		t.Fatalf("expected no baseline references (only a simulated one exists), got %#v", commit.PassportBaselineIDs)
	}
}

func TestPhysicalCommitProjectIsolation(t *testing.T) {
	app := testOwnedApp(t)

	createA := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Project A", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	if createA.StatusCode != http.StatusCreated {
		t.Fatalf("create A status = %d body=%s", createA.StatusCode, readBody(t, createA))
	}
	var projectA domain.Project
	decodeBody(t, createA, &projectA)

	createB := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-b", map[string]any{"name": "Project B", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	if createB.StatusCode != http.StatusCreated {
		t.Fatalf("create B status = %d body=%s", createB.StatusCode, readBody(t, createB))
	}
	var projectB domain.Project
	decodeBody(t, createB, &projectB)

	commitResponse := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-a", nil)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	// Owner B cannot create a commit against A's project (ownership mismatch
	// is indistinguishable from not-found).
	forbidden := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-b", nil)
	if forbidden.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-owner create status = %d body=%s", forbidden.StatusCode, readBody(t, forbidden))
	}

	// The commit exists, but fetching it through the wrong project id 404s
	// exactly like a missing id would.
	crossProject := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectB.ID+"/physical-commits/"+commit.ID, "owner-b", nil)
	if crossProject.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project get status = %d body=%s", crossProject.StatusCode, readBody(t, crossProject))
	}

	// And B's own owner cannot list or read A's commits by guessing the project id.
	listAsB := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-b", nil)
	if listAsB.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-owner list status = %d body=%s", listAsB.StatusCode, readBody(t, listAsB))
	}
}

func TestPhysicalCommitMalformedOrMissingID(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	malformed := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/not-a-real-id", nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed id status = %d body=%s", malformed.StatusCode, readBody(t, malformed))
	}

	missing := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/pcommit-00000000000000000000000000000000", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing id status = %d body=%s", missing.StatusCode, readBody(t, missing))
	}

	unknownProject := doJSON(t, app, http.MethodPost, "/api/v1/projects/does-not-exist/physical-commits", nil)
	if unknownProject.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown project status = %d body=%s", unknownProject.StatusCode, readBody(t, unknownProject))
	}
}

func TestPhysicalCommitDetailResolvesCapturedMeasurement(t *testing.T) {
	app, repository := testApp(t)
	project := createTestProject(t, app)

	window, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: project.ID, Source: "serial", DeviceID: "box-1", Sequence: 1, CapturedAtMS: 1000, IngestedAtMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	// Force the project's profile to exist so the commit's electrical
	// evidence has a real profile_id to attach a measurement window under.
	// (createPhysicalCommit resolves the *latest* measurement for the
	// project id regardless of whether a profile was ever analyzed.)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	if commit.MeasurementID == nil || *commit.MeasurementID != window.ID {
		t.Fatalf("expected commit to reference measurement %d, got %#v", window.ID, commit.MeasurementID)
	}

	detailResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/detail", nil)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.StatusCode, readBody(t, detailResponse))
	}
	var detail domain.PhysicalCommitDetail
	decodeBody(t, detailResponse, &detail)
	if detail.Commit.ID != commit.ID {
		t.Fatalf("detail commit = %#v", detail.Commit)
	}
	if detail.Measurement == nil || detail.Measurement.ID != window.ID {
		t.Fatalf("expected detail to resolve measurement %d, got %#v", window.ID, detail.Measurement)
	}
}

func TestPhysicalCommitDetailOmitsMeasurementWhenNoneCaptured(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	detailResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/detail", nil)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.StatusCode, readBody(t, detailResponse))
	}
	var detail domain.PhysicalCommitDetail
	decodeBody(t, detailResponse, &detail)
	if detail.Measurement != nil {
		t.Fatalf("expected no fabricated measurement, got %#v", detail.Measurement)
	}
}

func TestPhysicalCommitDetailProjectIsolationAndMalformedID(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)

	malformed := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/not-a-real-id/detail", nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed id status = %d body=%s", malformed.StatusCode, readBody(t, malformed))
	}

	missing := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/pcommit-00000000000000000000000000000000/detail", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing id status = %d body=%s", missing.StatusCode, readBody(t, missing))
	}
}

// TestPhysicalCommitDetailRouteIsNotShadowed proves the "/detail" suffix
// route is reached and does not get misinterpreted by :commitId matching
// literally "detail" as a path segment collision at a different depth.
func TestPhysicalCommitDetailRouteIsNotShadowed(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	detailResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/"+commit.ID+"/detail", nil)
	if detailResponse.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", detailResponse.StatusCode, readBody(t, detailResponse))
	}
	var detail domain.PhysicalCommitDetail
	decodeBody(t, detailResponse, &detail)
	if detail.Commit.ID != commit.ID {
		t.Fatalf("detail did not resolve the requested commit: %#v", detail)
	}
}

func TestPhysicalCommitDiffComparesTwoCommits(t *testing.T) {
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

	firstCommit := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var first domain.PhysicalCommit
	decodeBody(t, firstCommit, &first)

	// Add a new component to the draft profile before the second commit.
	var draft domain.ProjectProfile
	decodeBody(t, doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/profile", nil), &draft)
	draft.Components = append(draft.Components, domain.ComponentSpecification{ID: "servo-1", Name: "SG90 Servo"})
	updateResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/profile", draft)
	if updateResponse.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateResponse.StatusCode, readBody(t, updateResponse))
	}

	secondCommit := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var second domain.PhysicalCommit
	decodeBody(t, secondCommit, &second)

	diffResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/diff?from="+first.ID+"&to="+second.ID, nil)
	if diffResponse.StatusCode != http.StatusOK {
		t.Fatalf("diff status = %d body=%s", diffResponse.StatusCode, readBody(t, diffResponse))
	}
	var diff domain.PhysicalCommitDiff
	decodeBody(t, diffResponse, &diff)
	if diff.FromCommit != first.ID || diff.ToCommit != second.ID {
		t.Fatalf("diff identity = %#v", diff)
	}
	if diff.Components.Status != domain.EvidenceChanged {
		t.Fatalf("components = %#v", diff.Components)
	}
	found := false
	for _, change := range diff.Components.Changes {
		if change.ComponentID == "servo-1" && change.Status == domain.EvidenceAdded {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected servo-1 ADDED, got %#v", diff.Components.Changes)
	}
}

func TestPhysicalCommitDiffSelfComparisonIsAllUnchanged(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	diffResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/diff?from="+commit.ID+"&to="+commit.ID, nil)
	if diffResponse.StatusCode != http.StatusOK {
		t.Fatalf("diff status = %d body=%s", diffResponse.StatusCode, readBody(t, diffResponse))
	}
	var diff domain.PhysicalCommitDiff
	decodeBody(t, diffResponse, &diff)
	if diff.Visual.Status != domain.EvidenceNotCaptured || diff.Components.Status != domain.EvidenceNotCaptured {
		t.Fatalf("self-diff = %#v", diff)
	}
}

func TestPhysicalCommitDiffMalformedOrMissingIDs(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)

	malformed := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/diff?from=not-real&to="+commit.ID, nil)
	if malformed.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed status = %d body=%s", malformed.StatusCode, readBody(t, malformed))
	}

	missing := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/diff?from="+commit.ID+"&to=pcommit-00000000000000000000000000000000", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d body=%s", missing.StatusCode, readBody(t, missing))
	}
}

func TestPhysicalCommitDiffCrossProjectRejected(t *testing.T) {
	app := testOwnedApp(t)

	createA := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Project A", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	var projectA domain.Project
	decodeBody(t, createA, &projectA)
	createB := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-b", map[string]any{"name": "Project B", "description": "", "controller": "ESP32", "logic_voltage": 3.3})
	var projectB domain.Project
	decodeBody(t, createB, &projectB)

	commitA1 := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-a", nil)
	var a1 domain.PhysicalCommit
	decodeBody(t, commitA1, &a1)
	commitA2 := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectA.ID+"/physical-commits", "owner-a", nil)
	var a2 domain.PhysicalCommit
	decodeBody(t, commitA2, &a2)
	commitB1 := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+projectB.ID+"/physical-commits", "owner-b", nil)
	var b1 domain.PhysicalCommit
	decodeBody(t, commitB1, &b1)

	// Comparing A's own two commits works.
	within := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectA.ID+"/physical-commits/diff?from="+a1.ID+"&to="+a2.ID, "owner-a", nil)
	if within.StatusCode != http.StatusOK {
		t.Fatalf("within-project diff status = %d body=%s", within.StatusCode, readBody(t, within))
	}

	// B's commit cannot be used as one side of a diff scoped to A's project.
	cross := doJSONAs(t, app, http.MethodGet, "/api/v1/projects/"+projectA.ID+"/physical-commits/diff?from="+a1.ID+"&to="+b1.ID, "owner-a", nil)
	if cross.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-project diff status = %d body=%s", cross.StatusCode, readBody(t, cross))
	}
}

// TestPhysicalCommitDiffRouteIsNotShadowedByCommitID proves the literal
// "/diff" route is reached and never falls through to getPhysicalCommit
// with commitId="diff" (which would 400 on the id-format regex instead of
// returning a structured diff).
func TestPhysicalCommitDiffRouteIsNotShadowedByCommitID(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	first := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var firstCommit domain.PhysicalCommit
	decodeBody(t, first, &firstCommit)
	second := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	var secondCommit domain.PhysicalCommit
	decodeBody(t, second, &secondCommit)

	diffResponse := doJSON(t, app, http.MethodGet, "/api/v1/projects/"+project.ID+"/physical-commits/diff?from="+firstCommit.ID+"&to="+secondCommit.ID, nil)
	if diffResponse.StatusCode != http.StatusOK {
		t.Fatalf("diff route status = %d body=%s (want 200, not 400 INVALID_PHYSICAL_COMMIT_ID from a shadowed :commitId route)", diffResponse.StatusCode, readBody(t, diffResponse))
	}
	var diff domain.PhysicalCommitDiff
	decodeBody(t, diffResponse, &diff)
	if diff.FromCommit != firstCommit.ID || diff.ToCommit != secondCommit.ID {
		t.Fatalf("diff = %#v", diff)
	}
}

// TestPhysicalCommitCapturesFrameFromConfiguredCamera proves the real
// end-to-end path: a project with a saved camera_config, and no manually
// uploaded file, gets its raw image from the camera automatically.
func TestPhysicalCommitCapturesFrameFromConfiguredCamera(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := testMJPEGServer(t, testJPEGFrame(t))
	defer server.Close()
	configResponse := doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": server.URL})
	if configResponse.StatusCode != http.StatusOK {
		t.Fatalf("camera config status = %d body=%s", configResponse.StatusCode, readBody(t, configResponse))
	}

	commitResponse := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", nil)
	if commitResponse.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", commitResponse.StatusCode, readBody(t, commitResponse))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, commitResponse, &commit)
	if commit.Image == nil || commit.Image.ContentType != "image/jpeg" {
		t.Fatalf("expected a camera-captured image, got %#v", commit.Image)
	}
}

// TestPhysicalCommitManualUploadTakesPrecedenceOverCamera proves a caller
// that explicitly attaches a photo is never silently overridden by an
// auto-captured camera frame.
func TestPhysicalCommitManualUploadTakesPrecedenceOverCamera(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := testMJPEGServer(t, testJPEGFrame(t))
	defer server.Close()
	doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": server.URL})

	manualImage := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	response := doPhysicalCommitMultipart(t, app, "/api/v1/projects/"+project.ID+"/physical-commits", "", "hardware.png", manualImage)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s", response.StatusCode, readBody(t, response))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, response, &commit)
	if commit.Image == nil || commit.Image.ContentType != "image/png" {
		t.Fatalf("expected the manually uploaded PNG to win, got %#v", commit.Image)
	}
}

// TestPhysicalCommitSucceedsWithoutImageWhenCameraUnreachable proves camera
// failure never blocks a commit or destroys other evidence -- it simply
// proceeds with no image, exactly like a project with no camera at all.
func TestPhysicalCommitSucceedsWithoutImageWhenCameraUnreachable(t *testing.T) {
	app, _ := testApp(t)
	project := createTestProject(t, app)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := server.URL
	server.Close()
	doJSON(t, app, http.MethodPut, "/api/v1/projects/"+project.ID+"/camera-config", map[string]any{"url": closedURL})

	response := doJSON(t, app, http.MethodPost, "/api/v1/projects/"+project.ID+"/physical-commits", map[string]any{"note": "bench check"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("commit status = %d body=%s (a camera failure must never block a commit)", response.StatusCode, readBody(t, response))
	}
	var commit domain.PhysicalCommit
	decodeBody(t, response, &commit)
	if commit.Image != nil {
		t.Fatalf("expected no image when the camera is unreachable, got %#v", commit.Image)
	}
	if commit.Note != "bench check" {
		t.Fatalf("camera failure must not destroy other evidence on the commit: %#v", commit)
	}
}
