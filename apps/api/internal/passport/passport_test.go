package passport

import (
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
)

func value(number float64) *float64 { return &number }

func healthyWindow(source string) domain.MeasurementWindow {
	return domain.MeasurementWindow{
		ID: 42, ProfileID: "ultrasonic-demo", Source: source, DeviceID: "box-1", CapturedAtMS: 1000, IngestedAtMS: 1500,
		Raw: domain.TelemetryEnvelope{SchemaVersion: 2, ProfileID: "ultrasonic-demo", DeviceID: "box-1", CapturedAtMS: 1000, WindowMS: 1000, Samples: []domain.TelemetrySample{
			{Probe: "P1", Mode: domain.ProbeModeAnalog, AnalogMV: []float64{2500, 2510}},
			{Probe: "P2", Mode: domain.ProbeModePulse, EdgeCount: 80000, RisingEdges: 40000, PeriodsUS: []float64{25}},
			{Probe: "P3", Mode: domain.ProbeModePulse, EdgeCount: 56, RisingEdges: 28, PeriodsUS: []float64{35211}},
		}},
		Analysis: domain.AnalysisResult{SchemaVersion: 2, ProfileID: "ultrasonic-demo", DeviceID: "box-1", CapturedAtMS: 1000, WindowMS: 1000, Probes: []domain.DerivedFacts{
			{Probe: "P1", Role: "POWER", Mode: domain.ProbeModeAnalog, AverageVoltage: value(5.01), Stable: true},
			{Probe: "P2", Role: "TRIG", Mode: domain.ProbeModePulse, FrequencyHz: value(40000), Stable: true},
			{Probe: "P3", Role: "ECHO", Mode: domain.ProbeModePulse, FrequencyHz: value(28.4), Stable: true},
		}},
	}
}

func TestPhysicalBaselineRequiresMatchingStableCapture(t *testing.T) {
	profile := profiles.UltrasonicDemo()
	window := healthyWindow("serial")
	baseline, err := BuildKnownGood(profile, window, "  sensor responds  ")
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Source != domain.BaselinePhysical || baseline.Probes[0].Trusted.Status != domain.BaselineUserConfirmedHealthy || baseline.Note != "sensor responds" {
		t.Fatalf("baseline = %+v", baseline)
	}
	unstable := window
	unstable.Analysis.Probes = append([]domain.DerivedFacts(nil), window.Analysis.Probes...)
	unstable.Analysis.Probes[2].DropoutEvents = 12
	unstable.Analysis.Probes[2].Stable = false
	if _, err := BuildKnownGood(profile, unstable, ""); err == nil {
		t.Fatal("unstable capture was accepted")
	}
	outOfSpec := window
	outOfSpec.Analysis.Probes = append([]domain.DerivedFacts(nil), window.Analysis.Probes...)
	outOfSpec.Analysis.Probes[0].AverageVoltage = value(4.2)
	if _, err := BuildKnownGood(profile, outOfSpec, ""); err == nil {
		t.Fatal("out-of-spec voltage was accepted as known good")
	}
	mismatch := window
	mismatch.Raw.ProfileID = "other"
	if _, err := BuildKnownGood(profile, mismatch, ""); err == nil {
		t.Fatal("mismatched raw profile was accepted")
	}
	window.Source = "browser"
	if _, err := BuildKnownGood(profile, window, ""); err == nil {
		t.Fatal("browser fallback was accepted")
	}
	zeroClock := healthyWindow("serial")
	zeroClock.CapturedAtMS = 0
	zeroClock.Raw.CapturedAtMS = 0
	zeroClock.Analysis.CapturedAtMS = 0
	if _, err := BuildKnownGood(profile, zeroClock, ""); err != nil {
		t.Fatalf("ESP32 capture without wall clock rejected: %v", err)
	}
}

func TestSimulatedBaselineCannotBecomePhysicalEvidence(t *testing.T) {
	profile := profiles.UltrasonicDemo()
	baseline, err := BuildKnownGood(profile, healthyWindow("simulator"), "")
	if err != nil {
		t.Fatal(err)
	}
	physical := ApplyKnownGood(profile, domain.BaselinePhysical, &baseline)
	if physical.Probes[0].Baseline != nil || physical.Probes[1].Baseline != nil {
		t.Fatal("seeded or simulated baseline leaked into physical profile")
	}
	simulated := ApplyKnownGood(profile, domain.BaselineSimulated, &baseline)
	if simulated.Probes[0].Baseline == nil || simulated.Probes[0].Baseline.AverageVoltage == nil {
		t.Fatal("simulated baseline did not apply to simulator")
	}
}

func TestPassportStatusRequiresLaterMatchingCapture(t *testing.T) {
	profile := profiles.UltrasonicDemo()
	baseline, err := BuildKnownGood(profile, healthyWindow("serial"), "")
	if err != nil {
		t.Fatal(err)
	}
	first := healthyWindow("serial")
	if status, _ := CurrentStatus(profile, &first, &baseline, true); status != domain.PassportNeedsVerification {
		t.Fatalf("same capture status = %s", status)
	}
	later := healthyWindow("serial")
	later.ID = 43
	later.CapturedAtMS = 2000
	if status, _ := CurrentStatus(profile, &later, &baseline, true); status != domain.PassportHealthy {
		t.Fatalf("matching capture status = %s", status)
	}
	revised := profile
	revised.Version++
	if status, _ := CurrentStatus(revised, &later, &baseline, true); status != domain.PassportNeedsVerification {
		t.Fatalf("outdated profile baseline verified current capture: %s", status)
	}
	later.Analysis.Probes = append([]domain.DerivedFacts(nil), later.Analysis.Probes...)
	later.Analysis.Probes[0].AverageVoltage = value(5.4)
	if status, _ := CurrentStatus(profile, &later, &baseline, true); status != domain.PassportDeviation {
		t.Fatalf("deviating capture status = %s", status)
	}
	later.Analysis.Probes[0].AverageVoltage = value(5.01)
	later.Analysis.Probes[1].FrequencyHz = value(42000)
	if status, _ := CurrentStatus(profile, &later, &baseline, true); status != domain.PassportDeviation {
		t.Fatalf("out-of-spec frequency under 10%% baseline tolerance was accepted: %s", status)
	}
	later.Source = "simulator"
	if status, _ := CurrentStatus(profile, &later, &baseline, true); status != domain.PassportNeedsVerification {
		t.Fatalf("simulated capture verified physical baseline: %s", status)
	}
}
