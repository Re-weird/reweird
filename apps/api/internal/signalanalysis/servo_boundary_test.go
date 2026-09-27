package signalanalysis

import (
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"testing"
)

// Firmware boundary handling must work with the existing strict analyzer;
// no changes to reliability policy or physical expectations are required.
func TestServoCompletePairsRemainTrustedAtWindowBoundary(t *testing.T) {
	profile := domain.ProjectProfile{ID: "servo-boundary", Confirmed: true, Probes: []domain.ProbeConfiguration{{
		Probe: "P4", Role: "SERVO", Mode: domain.ProbeModeDigital,
		SafeMeasurement: domain.SafeMeasurementConfig{MaxPinVoltage: 3.3, InputScale: 1},
		Expected:        domain.ExpectedSignal{Required: true, NominalFrequencyHz: f64(50), MinFrequencyHz: f64(45), MaxFrequencyHz: f64(55), MinPulseWidthUS: f64(450), MaxPulseWidthUS: f64(2450)},
	}}}
	for _, rising := range []uint32{50, 51, 52} {
		sample := domain.TelemetrySample{Probe: "P4", Mode: domain.ProbeModeDigital, RisingEdges: rising, FallingEdges: 50, EdgeCount: rising + 50, MaxGapUS: 19514}
		for i := 0; i < 32; i++ {
			sample.PeriodsUS = append(sample.PeriodsUS, 20000)
			sample.HighPulseWidthsUS = append(sample.HighPulseWidthsUS, 486)
		}
		result, err := New().Analyze(domain.TelemetryEnvelope{SchemaVersion: 2, DeviceID: "physical-fixture", ProfileID: profile.ID, WindowMS: 1001, Samples: []domain.TelemetrySample{sample}}, profile)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := result.Probe("P4")
		if rising == 52 {
			if !p.CaptureUnreliable {
				t.Fatal("two unmatched edges must still fail reliability")
			}
			continue
		}
		if p.CaptureUnreliable || p.FrequencyHz == nil || *p.FrequencyHz != 50 || p.AveragePulseWidthUS == nil || *p.AveragePulseWidthUS != 486 {
			t.Fatalf("%d/50 valid boundary capture rejected: %+v", rising, p)
		}
	}
}
