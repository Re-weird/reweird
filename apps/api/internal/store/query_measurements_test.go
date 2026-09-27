package store

import (
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func windowFixture(profileID, source, deviceID string, sequence uint64, capturedAtMS int64, probe string) domain.MeasurementWindow {
	return domain.MeasurementWindow{
		ProfileID: profileID, Source: source, DeviceID: deviceID, Sequence: sequence, CapturedAtMS: capturedAtMS,
		Raw: domain.TelemetryEnvelope{
			ProfileID: profileID, DeviceID: deviceID, Sequence: sequence, CapturedAtMS: capturedAtMS,
			Samples: []domain.TelemetrySample{{Probe: probe, Mode: domain.ProbeModeDigital}},
		},
	}
}

func TestQueryMeasurementsFiltersByDeviceAndSource(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	if _, err := repository.SaveMeasurement(windowFixture("profile-a", "simulator", "device-1", 1, 1000, "P1")); err != nil {
		t.Fatalf("SaveMeasurement() error = %v", err)
	}
	if _, err := repository.SaveMeasurement(windowFixture("profile-a", "serial", "device-2", 1, 2000, "P1")); err != nil {
		t.Fatalf("SaveMeasurement() error = %v", err)
	}

	byDevice, err := repository.QueryMeasurements(domain.MeasurementQuery{DeviceID: "device-1"})
	if err != nil {
		t.Fatalf("QueryMeasurements(device) error = %v", err)
	}
	if len(byDevice) != 1 || byDevice[0].DeviceID != "device-1" {
		t.Fatalf("QueryMeasurements(device_id=device-1) = %+v, want exactly the device-1 window", byDevice)
	}

	bySource, err := repository.QueryMeasurements(domain.MeasurementQuery{Source: "serial"})
	if err != nil {
		t.Fatalf("QueryMeasurements(source) error = %v", err)
	}
	if len(bySource) != 1 || bySource[0].Source != "serial" {
		t.Fatalf("QueryMeasurements(source=serial) = %+v, want exactly the serial window", bySource)
	}
}

func TestQueryMeasurementsFiltersByTimeRangeAndOrdersNewestFirst(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	for _, capturedAtMS := range []int64{1000, 2000, 3000} {
		if _, err := repository.SaveMeasurement(windowFixture("profile-a", "simulator", "device-1", uint64(capturedAtMS), capturedAtMS, "P1")); err != nil {
			t.Fatalf("SaveMeasurement() error = %v", err)
		}
	}

	since := int64(1500)
	until := int64(2500)
	windows, err := repository.QueryMeasurements(domain.MeasurementQuery{ProfileID: "profile-a", SinceMS: &since, UntilMS: &until})
	if err != nil {
		t.Fatalf("QueryMeasurements(time range) error = %v", err)
	}
	if len(windows) != 1 || windows[0].CapturedAtMS != 2000 {
		t.Fatalf("QueryMeasurements(time range) = %+v, want exactly the window at t=2000", windows)
	}

	all, err := repository.QueryMeasurements(domain.MeasurementQuery{ProfileID: "profile-a"})
	if err != nil {
		t.Fatalf("QueryMeasurements() error = %v", err)
	}
	if len(all) != 3 || all[0].CapturedAtMS != 3000 || all[2].CapturedAtMS != 1000 {
		t.Fatalf("QueryMeasurements() ordering = %+v, want newest-first (3000, 2000, 1000)", all)
	}
}

func TestQueryMeasurementsFiltersByProbe(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	if _, err := repository.SaveMeasurement(windowFixture("profile-a", "simulator", "device-1", 1, 1000, "P1")); err != nil {
		t.Fatalf("SaveMeasurement() error = %v", err)
	}
	if _, err := repository.SaveMeasurement(windowFixture("profile-a", "simulator", "device-1", 2, 2000, "P2")); err != nil {
		t.Fatalf("SaveMeasurement() error = %v", err)
	}

	windows, err := repository.QueryMeasurements(domain.MeasurementQuery{ProfileID: "profile-a", Probe: "P2"})
	if err != nil {
		t.Fatalf("QueryMeasurements(probe) error = %v", err)
	}
	if len(windows) != 1 || windows[0].Sequence != 2 {
		t.Fatalf("QueryMeasurements(probe=P2) = %+v, want exactly the P2 window", windows)
	}
}

func TestQueryMeasurementsLimitIsBounded(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	for sequence := uint64(1); sequence <= 5; sequence++ {
		if _, err := repository.SaveMeasurement(windowFixture("profile-a", "simulator", "device-1", sequence, int64(sequence)*1000, "P1")); err != nil {
			t.Fatalf("SaveMeasurement() error = %v", err)
		}
	}

	windows, err := repository.QueryMeasurements(domain.MeasurementQuery{ProfileID: "profile-a", Limit: 2})
	if err != nil {
		t.Fatalf("QueryMeasurements(limit=2) error = %v", err)
	}
	if len(windows) != 2 {
		t.Fatalf("len(windows) = %d, want 2 (limit enforced)", len(windows))
	}

	// A limit outside [1, 200] must fall back to the default, never an
	// unbounded scan.
	windows, err = repository.QueryMeasurements(domain.MeasurementQuery{ProfileID: "profile-a", Limit: 1_000_000})
	if err != nil {
		t.Fatalf("QueryMeasurements(huge limit) error = %v", err)
	}
	if len(windows) > 200 {
		t.Fatalf("len(windows) = %d, want <= 200", len(windows))
	}
}
