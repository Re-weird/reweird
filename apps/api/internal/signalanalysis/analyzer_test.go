package signalanalysis

import (
	"math"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestAnalyzerDerivesElectricalFacts(t *testing.T) {
	profile := domain.ProjectProfile{
		ID: "analysis-test", ProjectName: "Analysis", Controller: "ESP32", LogicVoltage: 3.3, Confirmed: true,
		Probes: []domain.ProbeConfiguration{
			{
				Probe: "P1", Role: "RAIL", Mode: domain.ProbeModeAnalog, IsPowerRail: true,
				Expected:        domain.ExpectedSignal{SignalType: "voltage", Required: true, Stable: true, MinVoltage: f64(4.75), MaxVoltage: f64(5.25), VoltageTolerancePct: 2},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 2},
			},
			{
				Probe: "P2", Role: "PWM", Mode: domain.ProbeModePulse,
				Expected:        domain.ExpectedSignal{SignalType: "pwm", Required: true, Stable: true, NominalFrequencyHz: f64(1000), MaxDropouts: 0},
				SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1},
			},
		},
	}
	envelope := domain.TelemetryEnvelope{
		SchemaVersion: 1, DeviceID: "analysis-device", WindowMS: 1000,
		Samples: []domain.TelemetrySample{
			{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{2490, 2500, 2510}},
			{Probe: "P2", Mode: domain.ProbeModePulse, EdgeCount: 2000, RisingEdges: 1000, FallingEdges: 1000, PeriodsUS: []float64{990, 1000, 1010}, HighPulseWidthsUS: []float64{490, 500, 510}},
		},
	}

	result, err := New().Analyze(envelope, profile)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	rail, _ := result.Probe("P1")
	if rail.AverageVoltage == nil || math.Abs(*rail.AverageVoltage-5) > 0.001 {
		t.Fatalf("average voltage = %v, want 5.0", rail.AverageVoltage)
	}
	if rail.VoltageVariation == nil || math.Abs(*rail.VoltageVariation-0.04) > 0.001 {
		t.Fatalf("voltage variation = %v, want 0.04", rail.VoltageVariation)
	}
	pwm, _ := result.Probe("P2")
	if pwm.FrequencyHz == nil || math.Abs(*pwm.FrequencyHz-1000) > 0.01 {
		t.Fatalf("frequency = %v, want 1000 Hz", pwm.FrequencyHz)
	}
	if pwm.DutyCyclePercent == nil || math.Abs(*pwm.DutyCyclePercent-50) > 0.01 {
		t.Fatalf("duty cycle = %v, want 50%%", pwm.DutyCyclePercent)
	}
	if pwm.JitterUS == nil || *pwm.JitterUS <= 0 {
		t.Fatalf("jitter = %v, want positive", pwm.JitterUS)
	}
}

func TestAnalyzerFindsSimultaneousFailureBuckets(t *testing.T) {
	configuration := func(probe string) domain.ProbeConfiguration {
		return domain.ProbeConfiguration{
			Probe: probe, Role: probe, Mode: domain.ProbeModePulse,
			Expected:        domain.ExpectedSignal{SignalType: "pulse", Required: true, NominalFrequencyHz: f64(4)},
			SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1},
		}
	}
	profile := domain.ProjectProfile{ID: "shared-failure", ProjectName: "Shared", Controller: "ESP32", LogicVoltage: 3.3, Confirmed: true, Probes: []domain.ProbeConfiguration{configuration("P2"), configuration("P3")}}
	envelope := domain.TelemetryEnvelope{
		SchemaVersion: 1, DeviceID: "analysis-device", WindowMS: 1000,
		Samples: []domain.TelemetrySample{
			{Probe: "P2", Mode: domain.ProbeModePulse, EdgeCount: 6, RisingEdges: 3, FallingEdges: 3, ActivityCounts: []float64{1, 0, 1}},
			{Probe: "P3", Mode: domain.ProbeModePulse, EdgeCount: 6, RisingEdges: 3, FallingEdges: 3, ActivityCounts: []float64{1, 0, 1}},
		},
	}

	result, err := New().Analyze(envelope, profile)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(result.SimultaneousDropoutGroups) != 1 || len(result.SimultaneousDropoutGroups[0]) != 2 {
		t.Fatalf("SimultaneousDropoutGroups = %#v", result.SimultaneousDropoutGroups)
	}
}

func f64(value float64) *float64 { return &value }
