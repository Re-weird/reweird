package store

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestMeasurementRetentionFailsClosedWithoutDeleting(t *testing.T) {
	t.Setenv("MAX_MEASUREMENT_WINDOWS", "100")
	repository, err := Open(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	for sequence := int64(1); sequence <= 100; sequence++ {
		_, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: "p", Source: "test", DeviceID: "d", Sequence: uint64(sequence), CapturedAtMS: sequence})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: "p", Source: "test", DeviceID: "d", Sequence: 101, CapturedAtMS: 101}); err == nil || !strings.Contains(err.Error(), "retention limit") {
		t.Fatalf("expected retention error, got %v", err)
	}
	count, err := repository.MeasurementWindowCount()
	if err != nil || count != 100 {
		t.Fatalf("count %d: %v", count, err)
	}
	if _, err := repository.SaveMeasurement(domain.MeasurementWindow{ProfileID: "p", Source: "test", DeviceID: "d", Sequence: 1, CapturedAtMS: 1}); err != nil {
		t.Fatalf("existing window update blocked: %v", err)
	}
}
