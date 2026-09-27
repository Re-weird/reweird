package telemetrystore

import (
	"context"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func sampleWindow(profileID, deviceID string, sequence uint64, capturedAtMS int64) domain.MeasurementWindow {
	state := 1
	return domain.MeasurementWindow{
		ProfileID: profileID, DeviceID: deviceID, Source: "test", Sequence: sequence, CapturedAtMS: capturedAtMS,
		Raw: domain.TelemetryEnvelope{
			ProfileID: profileID, DeviceID: deviceID, Sequence: sequence, CapturedAtMS: capturedAtMS,
			Samples: []domain.TelemetrySample{
				{Probe: "P1", Mode: domain.ProbeModeDigital, State: &state, EdgeCount: 4},
				{Probe: "P2", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{3.3, 3.31, 3.29}},
			},
		},
	}
}

func TestFlattenMeasurementWindowProducesOneRowPerProbe(t *testing.T) {
	records := domain.FlattenMeasurementWindow(sampleWindow("profile-1", "device-1", 1, 1000))
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Probe != "P1" || records[0].State == nil || *records[0].State != 1 {
		t.Fatalf("records[0] = %+v, want P1 digital state=1", records[0])
	}
	if records[1].Probe != "P2" || records[1].AvgAnalogMV == nil {
		t.Fatalf("records[1] = %+v, want P2 with an averaged analog value", records[1])
	}
	got := *records[1].AvgAnalogMV
	if got < 3.29 || got > 3.31 {
		t.Fatalf("AvgAnalogMV = %v, want ~3.3", got)
	}
}

func TestFlattenMeasurementWindowOmitsAbsentFields(t *testing.T) {
	records := domain.FlattenMeasurementWindow(sampleWindow("p", "d", 1, 1000))
	if records[0].AvgAnalogMV != nil {
		t.Fatalf("P1 (digital) AvgAnalogMV = %v, want nil", records[0].AvgAnalogMV)
	}
	if records[1].State != nil {
		t.Fatalf("P2 (analog) State = %v, want nil", records[1].State)
	}
}

func TestMemoryStoreInsertAndQueryByProbe(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	if err := store.Insert(ctx, sampleWindow("profile-1", "device-1", 1, 1000)); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	if err := store.Insert(ctx, sampleWindow("profile-1", "device-1", 2, 2000)); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}
	records, err := store.Query(ctx, domain.TelemetryQuery{Probe: "P1"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2 (one P1 row per window)", len(records))
	}
	for _, record := range records {
		if record.Probe != "P1" {
			t.Fatalf("Query(Probe=P1) returned probe %q", record.Probe)
		}
	}
}

func TestMemoryStoreQueryOrdersNewestFirstAndRespectsTimeRange(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	_ = store.Insert(ctx, sampleWindow("p", "d", 1, 1000))
	_ = store.Insert(ctx, sampleWindow("p", "d", 2, 2000))
	_ = store.Insert(ctx, sampleWindow("p", "d", 3, 3000))

	since := int64(1500)
	until := int64(2500)
	records, err := store.Query(ctx, domain.TelemetryQuery{Probe: "P1", SinceMS: &since, UntilMS: &until})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(records) != 1 || records[0].TimeMS != 2000 {
		t.Fatalf("Query(time range) = %+v, want exactly the window at t=2000", records)
	}
}

func TestMemoryStoreQueryLimitIsBounded(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	for sequence := uint64(1); sequence <= 5; sequence++ {
		_ = store.Insert(ctx, sampleWindow("p", "d", sequence, int64(sequence)*1000))
	}
	records, err := store.Query(ctx, domain.TelemetryQuery{Probe: "P1", Limit: 2})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2 (limit enforced)", len(records))
	}
	// Requesting far more than MaxQueryLimit must still be clamped, never
	// an unbounded scan.
	records, err = store.Query(ctx, domain.TelemetryQuery{Probe: "P1", Limit: 1_000_000})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(records) > MaxQueryLimit {
		t.Fatalf("len(records) = %d, want <= MaxQueryLimit (%d)", len(records), MaxQueryLimit)
	}
}
