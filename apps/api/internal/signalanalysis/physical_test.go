package signalanalysis

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
)

func analyzeFrame(t *testing.T, envelope domain.TelemetryEnvelope, profile domain.ProjectProfile) domain.AnalysisResult {
	t.Helper()
	result, err := New().Analyze(envelope, profile)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func probeFacts(t *testing.T, result domain.AnalysisResult, probe string) domain.DerivedFacts {
	t.Helper()
	facts, ok := result.Probe(probe)
	if !ok {
		t.Fatalf("%s missing", probe)
	}
	return facts
}

func TestSlowPhysicalSignalsAreNotCountedAsBucketDropouts(t *testing.T) {
	result := analyzeFrame(t, physicalfixture.Frame(1), physicalfixture.Profile())
	for _, probe := range []string{"P2", "P3"} {
		facts := probeFacts(t, result, probe)
		if len(facts.FailureBuckets) != 0 || facts.DropoutEvents != 0 || !facts.Stable {
			t.Fatalf("%s at 3.8 Hz was treated as dropping out: %+v", probe, facts)
		}
	}
	if len(result.SimultaneousDropoutGroups) != 0 {
		t.Fatalf("empty 100 ms buckets of a 3.8 Hz signal produced a shared failure: %v", result.SimultaneousDropoutGroups)
	}
	if result.ProfileVersion != 1 {
		t.Fatalf("analysis lost its profile revision: %d", result.ProfileVersion)
	}
}

func TestLegacyCaptureIsReportedUnreliableWithoutEditingRawValues(t *testing.T) {
	envelope := physicalfixture.Frame(1)
	envelope.Samples[1] = physicalfixture.LegacyTrigSample()
	original := physicalfixture.LegacyTrigSample()
	result := analyzeFrame(t, envelope, physicalfixture.Profile())
	facts := probeFacts(t, result, "P2")
	if !facts.CaptureUnreliable || facts.Stable {
		t.Fatalf("inconsistent legacy capture was trusted: %+v", facts)
	}
	if !strings.Contains(strings.Join(facts.CaptureIssues, " "), "spans more than one capture window") {
		t.Fatalf("cross-window pulse timing was not reported: %v", facts.CaptureIssues)
	}
	if !reflect.DeepEqual(envelope.Samples[1], original) {
		t.Fatal("analysis modified the raw sample")
	}
	if facts.MaximumPulseWidthUS != nil || facts.FrequencyHz != nil || facts.AveragePulseWidthUS != nil {
		t.Fatalf("invalid raw timing leaked into derived facts: %+v", facts)
	}
	if len(result.SimultaneousDropoutGroups) != 0 {
		t.Fatalf("unreliable capture corroborated a shared failure: %v", result.SimultaneousDropoutGroups)
	}
}

func TestGapLongerThanWindowIsFlaggedNotClamped(t *testing.T) {
	envelope := physicalfixture.Frame(1)
	envelope.Samples[2].MaxGapUS = 3_091_975
	facts := probeFacts(t, analyzeFrame(t, envelope, physicalfixture.Profile()), "P3")
	if !facts.CaptureUnreliable || facts.MaximumGapUS == nil || *facts.MaximumGapUS != 3_091_975 {
		t.Fatalf("out-of-window gap was hidden or clamped: %+v", facts)
	}
	if facts.DropoutEvents != 0 {
		t.Fatalf("an untrustworthy gap was converted into %d dropouts", facts.DropoutEvents)
	}
}

func TestMisconfiguredFortyKilohertzTriggerStillFails(t *testing.T) {
	// The demo profile expects 40 kHz on TRIG. A real 3.8 Hz trigger must
	// keep failing against that expectation; the fix is the profile, not the
	// analyzer.
	demo := profiles.UltrasonicDemo()
	envelope := physicalfixture.Frame(1)
	envelope.ProfileID = demo.ID
	zero := 0
	envelope.Samples[4] = domain.TelemetrySample{Probe: "P5", Mode: domain.ProbeModeDigital, State: &zero}
	facts := probeFacts(t, analyzeFrame(t, envelope, demo), "P2")
	if facts.Stable || facts.DropoutEvents == 0 {
		t.Fatalf("real TRIG passed a 40 kHz expectation: %+v", facts)
	}
}

func TestPulseWidthExpectations(t *testing.T) {
	profile := physicalfixture.Profile()
	healthy := probeFacts(t, analyzeFrame(t, physicalfixture.Frame(1), profile), "P2")
	if healthy.PulseWidthOutOfRange || !healthy.Stable {
		t.Fatalf("10 µs trigger failed a 5–20 µs expectation: %+v", healthy)
	}
	long := physicalfixture.Frame(1)
	long.Samples[1].HighPulseWidthsUS = []float64{10, 10, 31, 10}
	facts := probeFacts(t, analyzeFrame(t, long, profile), "P2")
	if !facts.PulseWidthOutOfRange || facts.Stable {
		t.Fatalf("31 µs trigger passed a 5–20 µs expectation: %+v", facts)
	}
}

func TestGlitchFloorDoesNotHideMeasurableTimingDrift(t *testing.T) {
	for _, width := range []float64{0.1, 4, 11.4} {
		frame := physicalfixture.Frame(1)
		frame.Samples[1].HighPulseWidthsUS = []float64{width, width, width, width}
		facts := probeFacts(t, analyzeFrame(t, frame, physicalfixture.Profile()), "P2")
		if width == 0.1 {
			if !facts.CaptureUnreliable || facts.FrequencyHz != nil || facts.AveragePulseWidthUS != nil || !strings.Contains(strings.Join(facts.CaptureIssues, " "), "minimum-valid pulse floor") {
				t.Fatalf("glitches were measured as pulses: %+v", facts)
			}
		} else if facts.CaptureUnreliable || facts.FrequencyHz == nil || facts.AveragePulseWidthUS == nil || facts.PulseWidthOutOfRange != (width == 4) {
			t.Fatalf("legitimate timing or measurable drift was hidden: %+v", facts)
		}
	}
}

func TestMissingPulsesInRegularTrainAreDropouts(t *testing.T) {
	envelope := physicalfixture.Frame(1)
	servo := &envelope.Samples[3]
	servo.RisingEdges, servo.FallingEdges, servo.EdgeCount = 47, 47, 94
	servo.MaxGapUS = 79_000 // four servo periods without a pulse
	facts := probeFacts(t, analyzeFrame(t, envelope, physicalfixture.Profile()), "P4")
	if facts.DropoutEvents < 3 || facts.Stable {
		t.Fatalf("missing servo pulses were not counted: %+v", facts)
	}
}

// The frames in testdata were emitted by the real firmware/esp32/src/main.cpp
// compiled for the host against a simulated clock and ideal bench signals
// (see firmware/esp32/host-test/host_harness.cpp). They prove the firmware's
// window accounting produces frames the backend accepts as consistent; they
// say nothing about the physical wiring.
func TestFirmwareHarnessFramesAreConsistent(t *testing.T) {
	data, err := os.ReadFile("testdata/firmware_harness_frames.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	profile := physicalfixture.Profile()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected several frames, got %d", len(lines))
	}
	for _, line := range lines {
		var envelope domain.TelemetryEnvelope
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		envelope.DeviceID = physicalfixture.DeviceID
		result := analyzeFrame(t, envelope, profile)
		for _, facts := range result.Probes {
			if facts.CaptureUnreliable {
				t.Fatalf("sequence %d %s: %v", envelope.Sequence, facts.Probe, facts.CaptureIssues)
			}
			if facts.MaximumGapUS != nil && *facts.MaximumGapUS > float64(envelope.WindowMS)*1000 {
				t.Fatalf("sequence %d %s gap %v exceeds window", envelope.Sequence, facts.Probe, *facts.MaximumGapUS)
			}
		}
		trig, _ := result.Probe("P2")
		if trig.AveragePulseWidthUS == nil || *trig.AveragePulseWidthUS != 10 || trig.PulseWidthOutOfRange {
			t.Fatalf("hardware-captured 10 us trigger = %+v", trig)
		}
	}
}
