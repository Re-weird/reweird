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
