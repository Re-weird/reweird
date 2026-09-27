package demodata

import (
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalgit"
	"github.com/re-weird/reweird/apps/api/internal/store"
)

func openTestStore(t *testing.T) (*store.SQLiteStore, string) {
	t.Helper()
	root := t.TempDir()
	repository, err := store.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository, filepath.Join(root, "uploads")
}

func TestSeedCreatesCanonicalProjectAndThreeCommits(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	project, err := repository.GetProject(PhysicalGitDemoProjectID)
	if err != nil || project == nil || project.OwnerID != "" {
		t.Fatalf("project = %#v, err = %v", project, err)
	}
	items, err := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("commit count = %d, want 3", len(items))
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("repeated seeding produced %d commits, want 3", len(items))
	}
}

func findCommit(t *testing.T, items []domain.PhysicalCommit, displayID string) domain.PhysicalCommit {
	t.Helper()
	for _, item := range items {
		if item.DisplayID == displayID {
			return item
		}
	}
	t.Fatalf("no commit with display id %s among %#v", displayID, items)
	return domain.PhysicalCommit{}
}

func TestHW001HasWorkingStateEvidence(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw1 := findCommit(t, items, "HW-001")
	if hw1.Image == nil || hw1.ProfileSnapshot == nil || hw1.MeasurementID == nil {
		t.Fatalf("hw1 = %#v", hw1)
	}
	if len(hw1.ProfileSnapshot.Components) != 2 {
		t.Fatalf("hw1 components = %#v", hw1.ProfileSnapshot.Components)
	}
	measurement, err := repository.GetMeasurement(*hw1.MeasurementID)
	if err != nil || measurement == nil {
		t.Fatalf("measurement = %#v, err = %v", measurement, err)
	}
	echo, ok := measurement.Analysis.Probe("ECHO")
	if !ok || echo.MissingExpectedActivity || echo.AveragePulseWidthUS == nil || *echo.AveragePulseWidthUS != 1420 {
		t.Fatalf("hw1 echo facts = %#v", echo)
	}
}

func TestHW002HasChangedStateEvidence(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw2 := findCommit(t, items, "HW-002")
	if len(hw2.ProfileSnapshot.Components) != 3 {
		t.Fatalf("hw2 components = %#v", hw2.ProfileSnapshot.Components)
	}
	var echoConnection *domain.ProfileConnection
	for index := range hw2.ProfileSnapshot.Connections {
		if hw2.ProfileSnapshot.Connections[index].Role == "ECHO" {
			echoConnection = &hw2.ProfileSnapshot.Connections[index]
		}
	}
	if echoConnection == nil || echoConnection.GPIO == nil || *echoConnection.GPIO != 19 {
		t.Fatalf("hw2 echo connection = %#v", echoConnection)
	}
	measurement, _ := repository.GetMeasurement(*hw2.MeasurementID)
	echo, _ := measurement.Analysis.Probe("ECHO")
	if !echo.MissingExpectedActivity {
		t.Fatalf("hw2 echo activity should be missing: %#v", echo)
	}
}

func TestHW003HasRestoredStateEvidence(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw3 := findCommit(t, items, "HW-003")
	if len(hw3.ProfileSnapshot.Components) != 2 {
		t.Fatalf("hw3 components = %#v", hw3.ProfileSnapshot.Components)
	}
	measurement, _ := repository.GetMeasurement(*hw3.MeasurementID)
	echo, _ := measurement.Analysis.Probe("ECHO")
	if echo.MissingExpectedActivity || echo.AveragePulseWidthUS == nil {
		t.Fatalf("hw3 echo facts = %#v", echo)
	}
}

func TestDemoDiffDetectsRealDifferences(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw1 := findCommit(t, items, "HW-001")
	hw2 := findCommit(t, items, "HW-002")
	fromMeasurement, _ := repository.GetMeasurement(*hw1.MeasurementID)
	toMeasurement, _ := repository.GetMeasurement(*hw2.MeasurementID)
	fromVision, err := repository.GetPhysicalCommitVisionAnalysis(PhysicalGitDemoProjectID, hw1.ID)
	if err != nil || fromVision == nil {
		t.Fatalf("fromVision = %#v, err = %v", fromVision, err)
	}
	toVision, _ := repository.GetPhysicalCommitVisionAnalysis(PhysicalGitDemoProjectID, hw2.ID)
	if fromVision.Provider != ProviderSimulatedDemo {
		t.Fatalf("provider = %q, want %q", fromVision.Provider, ProviderSimulatedDemo)
	}

	diff := physicalgit.Diff(hw1, hw2, fromMeasurement, toMeasurement, fromVision, toVision)
	if diff.Components.Status != domain.EvidenceChanged {
		t.Fatalf("components = %#v", diff.Components)
	}
	if diff.Circuit.Status != domain.EvidenceChanged {
		t.Fatalf("circuit = %#v", diff.Circuit)
	}
	if diff.Electrical.Status != domain.EvidenceChanged {
		t.Fatalf("electrical = %#v", diff.Electrical)
	}
	if diff.SemanticVisual.Status != domain.EvidenceChanged {
		t.Fatalf("semantic_visual = %#v (both sides have a simulated persisted analysis)", diff.SemanticVisual)
	}
}

func TestDemoRestoreProducesDeterministicGuidance(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw1 := findCommit(t, items, "HW-001")
	hw2 := findCommit(t, items, "HW-002")
	plan := physicalgit.Restore(hw1, &hw2, nil, nil, nil, nil)
	if plan.Components.Status != domain.RestoreActionRequired {
		t.Fatalf("components = %#v", plan.Components)
	}
	if plan.Circuit.Status != domain.RestoreActionRequired {
		t.Fatalf("circuit = %#v", plan.Circuit)
	}
}

func TestApplyRestorationAndApplyBreakFlipVerifyOutcome(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
	items, _ := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	hw1 := findCommit(t, items, "HW-001")

	// Right after seeding, live state is deliberately BROKEN.
	currentProfile, _ := repository.GetProfile(PhysicalGitDemoProjectID)
	latest, _ := repository.ListMeasurements(PhysicalGitDemoProjectID, 1)
	targetMeasurement, _ := repository.GetMeasurement(*hw1.MeasurementID)
	before := physicalgit.Verify(hw1, physicalgit.VerifyInput{CurrentProfile: currentProfile, TargetMeasurement: targetMeasurement, CurrentMeasurement: &latest[0]})
	if before.Overall != domain.VerifyNotSupported {
		t.Fatalf("before ApplyRestoration, overall = %s, want NOT_SUPPORTED", before.Overall)
	}

	if err := ApplyRestoration(repository); err != nil {
		t.Fatal(err)
	}
	currentProfile, _ = repository.GetProfile(PhysicalGitDemoProjectID)
	latest, _ = repository.ListMeasurements(PhysicalGitDemoProjectID, 1)
	after := physicalgit.Verify(hw1, physicalgit.VerifyInput{CurrentProfile: currentProfile, TargetMeasurement: targetMeasurement, CurrentMeasurement: &latest[0]})
	if after.Overall != domain.VerifySupported {
		t.Fatalf("after ApplyRestoration, overall = %s, want SUPPORTED", after.Overall)
	}

	if err := ApplyBreak(repository); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	if err != nil || len(items) != 3 {
		t.Fatalf("ApplyBreak must never touch commit history: items = %#v, err = %v", items, err)
	}
}

// TestSeedResumesAfterAPartialFailure simulates the exact failure this
// package hit in development: a prior Seed call created the project/profile
// but failed before any commit was written (e.g. an unrelated measurement-
// retention limit). Re-running Seed must complete the story, not skip
// seeding forever just because the project row already exists.
func TestSeedResumesAfterAPartialFailure(t *testing.T) {
	repository, uploadRoot := openTestStore(t)
	if err := repository.SaveProject(domain.Project{ID: PhysicalGitDemoProjectID, Name: "Ultrasonic Robot Demo", Controller: "ESP32", LogicVoltage: 3.3}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(buildProfile(false)); err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	if err != nil || len(items) != 0 {
		t.Fatalf("precondition: expected zero commits, got %d, err=%v", len(items), err)
	}

	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatalf("Seed did not resume after a partial failure: %v", err)
	}
	items, err = repository.ListPhysicalCommits(PhysicalGitDemoProjectID)
	if err != nil || len(items) != 3 {
		t.Fatalf("commit count = %d, want 3, err=%v", len(items), err)
	}
}

func TestSeedNeverInvokesGeminiOrGitHub(t *testing.T) {
	// Seed takes only a domain.Repository and an upload root -- there is no
	// vision.Analyzer, no HTTP client, and no GitHub client anywhere in its
	// signature, so it is structurally impossible for it to call either.
	repository, uploadRoot := openTestStore(t)
	if err := Seed(repository, uploadRoot); err != nil {
		t.Fatal(err)
	}
}
