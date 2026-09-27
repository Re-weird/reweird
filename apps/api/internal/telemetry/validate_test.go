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
	if err == nil || !strings.Contains(err.Error(), "firmware reports wrong-profile but this project expects right-profile") {
		t.Fatalf("ValidateForProfile() error = %v", err)
	}
}

func TestValidateForProfileNamesPhysicalModeMismatch(t *testing.T) {
	envelope := domain.TelemetryEnvelope{SchemaVersion: SchemaVersion, DeviceID: "reweird-001", ProfileID: "physical-project", WindowMS: 1000,
		Samples: []domain.TelemetrySample{{Probe: "P5", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{1650}}}}
	profile := domain.ProjectProfile{ID: "physical-project", Confirmed: true,
		Probes: []domain.ProbeConfiguration{{Probe: "P5", Role: "ZMPT", Mode: domain.ProbeModeDigital,
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1}}}}
	if err := ValidateForProfile(envelope, profile); err == nil || !strings.Contains(err.Error(), "P5 mode mismatch: received analog, profile expects digital") {
		t.Fatalf("mode mismatch error = %v", err)
	}
}
