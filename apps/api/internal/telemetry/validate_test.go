package telemetry

import (
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestValidateRejectsUnsafePayloads(t *testing.T) {
	state := 2
	envelope := domain.TelemetryEnvelope{
		SchemaVersion: SchemaVersion,
		DeviceID:      "reweird-001",
		ProfileID:     "test-profile",
		WindowMS:      1000,
		Samples: []domain.TelemetrySample{{
			Probe: "P1",
			Mode:  domain.ProbeModeDigital,
			State: &state,
		}},
	}

	err := Validate(envelope)
	if err == nil || !strings.Contains(err.Error(), "state must be 0 or 1") {
		t.Fatalf("Validate() error = %v, want invalid state", err)
	}
}

func TestValidateRejectsDuplicateProbes(t *testing.T) {
	envelope := domain.TelemetryEnvelope{
		SchemaVersion: SchemaVersion,
		DeviceID:      "reweird-001",
		ProfileID:     "test-profile",
		WindowMS:      1000,
		Samples: []domain.TelemetrySample{
			{Probe: "P2", Mode: domain.ProbeModePulse},
			{Probe: "P2", Mode: domain.ProbeModePulse},
		},
	}

	if err := Validate(envelope); err == nil {
		t.Fatal("Validate() accepted duplicate probe samples")
	}
}

func TestValidateForProfileRejectsCrossProjectTelemetry(t *testing.T) {
	envelope := domain.TelemetryEnvelope{
		SchemaVersion: SchemaVersion, DeviceID: "reweird-001", ProfileID: "wrong-profile", WindowMS: 1000,
		Samples: []domain.TelemetrySample{{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{1000}}},
	}
	profile := domain.ProjectProfile{
		ID: "right-profile", Confirmed: true,
		Probes: []domain.ProbeConfiguration{{Probe: "P1", Role: "RAIL", Mode: domain.ProbeModeAnalog, SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}}},
	}
	err := ValidateForProfile(envelope, profile)
	if err == nil || !strings.Contains(err.Error(), "does not match active profile") {
		t.Fatalf("ValidateForProfile() error = %v", err)
	}
}
