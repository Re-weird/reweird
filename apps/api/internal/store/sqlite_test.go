package store

import (
	"path/filepath"
	"testing"

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
