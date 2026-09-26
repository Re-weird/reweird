package store

import (
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
)

func TestProjectProfileRoundTrip(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	profile := profiles.UltrasonicDemo()
	if err := repository.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}
	stored, err := repository.GetProfile(profile.ID)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if stored == nil || stored.ProjectName != profile.ProjectName || !stored.Confirmed {
		t.Fatalf("GetProfile() = %#v", stored)
	}
	items, err := repository.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListProfiles() returned %d profiles, want 1", len(items))
	}
}

func TestProjectAndConfirmedProfilePersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reweird-persistence.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	project := domain.Project{ID: "persistent-project", Name: "Persistent Project", Controller: "ESP32", LogicVoltage: 3.3, AnalysisStatus: domain.AnalysisConfirmed, Code: &domain.ProjectCode{Filename: "main.ino", Text: "#define LED 2"}}
	profile := profiles.UltrasonicDemo()
	profile.ID = project.ID
	profile.ProjectID = project.ID
	profile.ProjectName = project.Name
	if err := repository.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	storedProject, err := reopened.GetProject(project.ID)
	if err != nil || storedProject == nil || storedProject.Code == nil || storedProject.Code.Text != project.Code.Text {
		t.Fatalf("project after reopen = %#v err=%v", storedProject, err)
	}
	storedProfile, err := reopened.GetProfile(profile.ID)
	if err != nil || storedProfile == nil || !storedProfile.Confirmed || storedProfile.ConfirmedBy != "user" {
		t.Fatalf("profile after reopen = %#v err=%v", storedProfile, err)
	}
}

func TestRawAndDerivedMeasurementWindowPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "measurements.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	window := domain.MeasurementWindow{
		ProfileID: "measurement-profile", Source: "simulator", DeviceID: "device-001", Sequence: 7, CapturedAtMS: 1234,
		Raw:      domain.TelemetryEnvelope{SchemaVersion: 2, DeviceID: "device-001", ProfileID: "measurement-profile", CapturedAtMS: 1234, WindowMS: 1000, Sequence: 7, Samples: []domain.TelemetrySample{{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{1000, 1100}}}},
		Analysis: domain.AnalysisResult{SchemaVersion: 2, DeviceID: "device-001", ProfileID: "measurement-profile", CapturedAtMS: 1234, WindowMS: 1000, Probes: []domain.DerivedFacts{{Probe: "P1", Role: "SUPPLY", Mode: domain.ProbeModeAnalog, Stable: true}}},
	}
	stored, err := repository.SaveMeasurement(window)
	if err != nil || stored.ID == 0 {
		t.Fatalf("SaveMeasurement() = %#v, %v", stored, err)
	}
	duplicate, err := repository.SaveMeasurement(window)
	if err != nil || duplicate.ID != stored.ID {
		t.Fatalf("same raw frame should reuse immutable window: %#v, %v", duplicate, err)
	}
	collision := window
	collision.Raw.Samples = append([]domain.TelemetrySample(nil), window.Raw.Samples...)
	collision.Raw.Samples[0].AnalogMV = []float64{1000, 1200}
	if _, err := repository.SaveMeasurement(collision); err == nil {
		t.Fatal("different raw frame reused existing source/device/sequence/time identity")
	}
	derivedCollision := window
	derivedCollision.Analysis.Probes = append([]domain.DerivedFacts(nil), window.Analysis.Probes...)
	derivedCollision.Analysis.Probes[0].Stable = false
	if _, err := repository.SaveMeasurement(derivedCollision); err == nil {
		t.Fatal("different derived evidence reused existing source/device/sequence/time identity")
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	windows, err := reopened.ListMeasurements("measurement-profile", 10)
	if err != nil || len(windows) != 1 {
		t.Fatalf("ListMeasurements() = %#v, %v", windows, err)
	}
	if windows[0].Raw.Samples[0].AnalogMV[1] != 1100 || windows[0].Analysis.Probes[0].Role != "SUPPLY" {
		t.Fatalf("measurement after reopen = %#v", windows[0])
	}
}

// Projects saved before visibility existed have no "visibility" key in their
// stored payload; both read paths must report them as private.
func TestLegacyProjectWithoutVisibilityReadsAsPrivate(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-legacy.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	legacy := `{"id":"legacy-rig","name":"Legacy rig","controller":"ESP32","logic_voltage":3.3,"analysis_status":"PENDING","created_at_ms":1,"updated_at_ms":1}`
	if _, err := repository.db.Exec(
		"INSERT INTO projects (id, payload, analysis_status, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))",
		"legacy-rig", legacy, "PENDING",
	); err != nil {
		t.Fatalf("insert legacy project: %v", err)
	}

	project, err := repository.GetProject("legacy-rig")
	if err != nil || project == nil {
		t.Fatalf("GetProject() = %v, %v", project, err)
	}
	if project.Visibility != domain.VisibilityPrivate {
		t.Fatalf("GetProject visibility = %q, want private", project.Visibility)
	}

	listed, err := repository.ListProjects()
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListProjects() = %v, %v", listed, err)
	}
	if listed[0].Visibility != domain.VisibilityPrivate {
		t.Fatalf("ListProjects visibility = %q, want private", listed[0].Visibility)
	}
}

func TestSetProjectVisibilityChangesOnlyVisibility(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-visibility.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	project := domain.Project{ID: "rig", Name: "Rig", Description: "original", Controller: "ESP32", LogicVoltage: 3.3, AnalysisStatus: domain.AnalysisPending, Visibility: domain.VisibilityPrivate}
	if err := repository.SaveProject(project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	stored, _ := repository.GetProject("rig")

	if err := repository.SetProjectVisibility("rig", domain.VisibilityPublic); err != nil {
		t.Fatalf("SetProjectVisibility() error = %v", err)
	}
	updated, err := repository.GetProject("rig")
	if err != nil || updated == nil {
		t.Fatalf("GetProject() = %v, %v", updated, err)
	}
	if updated.Visibility != domain.VisibilityPublic {
		t.Fatalf("visibility = %q, want public", updated.Visibility)
	}
	if updated.Description != "original" || updated.Controller != "ESP32" || updated.UpdatedAtMS != stored.UpdatedAtMS {
		t.Fatalf("other fields changed: %#v", updated)
	}

	if err := repository.SetProjectVisibility("missing", domain.VisibilityPublic); err == nil {
		t.Fatal("SetProjectVisibility on a missing project returned nil error")
	}
}
