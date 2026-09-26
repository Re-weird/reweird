package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestKnownGoodCapturePersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passport.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	window, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: "profile-a", Source: "serial", DeviceID: "box-a", Sequence: 1, CapturedAtMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	record, err := repository.SaveKnownGood(domain.KnownGoodBaseline{
		ProfileID: "profile-a", MeasurementID: window.ID, Source: domain.BaselinePhysical,
		DeviceID: "box-a", SavedAtMS: 2000, Probes: []domain.BaselineProbe{},
	})
	if err != nil || record.ID == 0 {
		t.Fatalf("save = %+v, %v", record, err)
	}
	if _, err := repository.SaveKnownGood(record); !errors.Is(err, domain.ErrKnownGoodExists) {
		t.Fatalf("duplicate capture error = %v", err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	latest, err := reopened.LatestKnownGood("profile-a", domain.BaselinePhysical, "box-a")
	if err != nil || latest == nil || latest.MeasurementID != window.ID {
		t.Fatalf("reopened baseline = %+v, %v", latest, err)
	}
	physical, err := reopened.HasPhysicalBaseline("profile-a")
	if err != nil || !physical {
		t.Fatalf("physical baseline = %v, %v", physical, err)
	}
}
