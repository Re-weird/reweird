package serialsource

import (
	"context"
	"testing"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestDecodeLineAcceptsVersionedTelemetry(t *testing.T) {
	line := []byte(`{"schema_version":1,"device_id":"reweird-001","captured_at_ms":1000,"uptime_ms":1000,"window_ms":1000,"sequence":1,"samples":[{"probe":"P1","mode":"analog","analog_mv":[1640,1650,1660]}]}`)

	envelope, err := DecodeLine(line)
	if err != nil {
		t.Fatalf("DecodeLine() error = %v", err)
	}
	if envelope.DeviceID != "reweird-001" || len(envelope.Samples) != 1 {
		t.Fatalf("DecodeLine() envelope = %#v", envelope)
	}
}

func TestDecodeLineRejectsUnknownFields(t *testing.T) {
	line := []byte(`{"schema_version":1,"device_id":"reweird-001","window_ms":1000,"samples":[{"probe":"P1","mode":"analog"}],"command":"drive_patch_high"}`)

	if _, err := DecodeLine(line); err == nil {
		t.Fatal("DecodeLine() accepted an unknown command field")
	}
}

func TestWaitNextRequiresANewerSequence(t *testing.T) {
	source := &Source{
		latest:     &domain.TelemetryEnvelope{Sequence: 4, WindowMS: 1000},
		receivedAt: time.Now(),
		updates:    make(chan struct{}, 1),
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		source.mu.Lock()
		source.latest = &domain.TelemetryEnvelope{Sequence: 5, WindowMS: 1000}
		source.receivedAt = time.Now()
		source.mu.Unlock()
		source.updates <- struct{}{}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	envelope, err := source.WaitNext(ctx, 4)
	if err != nil {
		t.Fatalf("WaitNext() error = %v", err)
	}
	if envelope.Sequence != 5 {
		t.Fatalf("WaitNext() sequence = %d, want 5", envelope.Sequence)
	}
}
