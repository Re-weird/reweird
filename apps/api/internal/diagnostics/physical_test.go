package diagnostics

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
)

func learnedProfile(t *testing.T) domain.ProjectProfile {
	t.Helper()
	profile := physicalfixture.Profile()
	analyzer := signalanalysis.New()
	run := make([]domain.MeasurementWindow, 0, passport.CalibrationWindows)
	for index := 0; index < passport.CalibrationWindows; index++ {
		envelope := physicalfixture.Frame(uint64(10 + index))
		analysis, err := analyzer.Analyze(envelope, profile)
		if err != nil {
			t.Fatal(err)
		}
		run = append(run, domain.MeasurementWindow{ID: int64(100 + index), ProfileID: profile.ID, Source: "serial", DeviceID: envelope.DeviceID, Sequence: envelope.Sequence, IngestedAtMS: int64(1000 + index), Raw: envelope, Analysis: analysis})
	}
	baseline, err := passport.LearnKnownGood(profile, run, "")
	if err != nil {
		t.Fatal(err)
	}
	return passport.ApplyKnownGood(profile, domain.BaselinePhysical, &baseline)
}

func TestCalibratedPhysicalCaptureShowsKnownGoodInsteadOfUnknown(t *testing.T) {
	engine := NewEngine(signalanalysis.New())
	session, err := engine.AnalyzeEnvelope(context.Background(), learnedProfile(t), domain.StageDiagnose, "serial", physicalfixture.Frame(50), nil)
	if err != nil {
		t.Fatal(err)
	}
	if session.Evidence.Baseline["status"] != domain.BaselineUserConfirmedHealthy || session.Evidence.Baseline["learned_windows"] != passport.CalibrationWindows {
		t.Fatalf("baseline evidence = %+v", session.Evidence.Baseline)
	}
	if hasRule(session.Evidence.RuleResults, "known-good-deviation", "fail") || hasFailure(session.Evidence.RuleResults) {
		t.Fatalf("matching capture failed: %+v", session.Evidence.RuleResults)
	}
}

func TestKnownGoodDeviationDrivesDiagnosisAndNextTest(t *testing.T) {
	profile := learnedProfile(t)
	frame := physicalfixture.Frame(50)
	frame.Samples[0].AnalogMV = []float64{2440, 2441, 2439, 2440}
	session, err := NewEngine(signalanalysis.New()).AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "serial", frame, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRuleForProbe(session.Evidence.RuleResults, "known-good-deviation", "P1") || !strings.Contains(session.Diagnosis.Headline, "Known Good") {
		t.Fatalf("diagnosis ignored the Known Good: %s %+v", session.Diagnosis.Headline, session.Evidence.RuleResults)
	}
	recommendation := testplanner.New().Recommend(profile, session.Analysis, session.ID)
	if recommendation.TestType != domain.TestBaselineComparison || recommendation.TargetProbes[0] != "P1" {
		t.Fatalf("next test = %+v", recommendation)
	}
}

func TestUnreliableCaptureBlocksCircuitConclusions(t *testing.T) {
	profile := physicalfixture.Profile()
	frame := physicalfixture.Frame(50)
	frame.Samples[1] = physicalfixture.LegacyTrigSample()
	session, err := NewEngine(signalanalysis.New()).AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "serial", frame, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(session.Diagnosis.Headline, "not trustworthy") || strings.Contains(session.Diagnosis.Headline, "Shared electrical") {
		t.Fatalf("diagnosis drew a circuit conclusion from an unreliable capture: %s", session.Diagnosis.Headline)
	}
	recommendation := testplanner.New().Recommend(profile, session.Analysis, session.ID)
	if recommendation.TestType != domain.TestRemeasure || recommendation.TargetProbes[0] != "P2" {
		t.Fatalf("next test = %+v", recommendation)
	}
}

// testdata/legacy_serial_frame_1866.json is a real frame from device
// reweird-3428B5AD4F7C running the previous interrupt-only firmware against
// the seeded ultrasonic-demo profile. The old analyzer called this "Shared
// electrical instability" at 82% because a 3.8 Hz ECHO leaves most 100 ms
// buckets empty and P2's gap spanned several windows.
func TestRealLegacyFrameNoLongerInventsSharedInstability(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy_serial_frame_1866.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope domain.TelemetryEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	session, err := NewEngine(signalanalysis.New()).AnalyzeEnvelope(context.Background(), profiles.UltrasonicDemo(), domain.StageDiagnose, "serial", envelope, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(session.Diagnosis.Headline, "Shared electrical") || len(session.Analysis.SimultaneousDropoutGroups) != 0 {
		t.Fatalf("slow ECHO buckets still produced a shared failure: %s %v", session.Diagnosis.Headline, session.Analysis.SimultaneousDropoutGroups)
	}
	trig, _ := session.Analysis.Probe("P2")
	if !trig.CaptureUnreliable || trig.MaximumGapUS == nil || *trig.MaximumGapUS != 1_052_749 {
		t.Fatalf("P2 gap longer than its window was not reported as-is: %+v", trig)
	}
	echo, _ := session.Analysis.Probe("P3")
	if echo.DropoutEvents != 0 || len(echo.FailureBuckets) != 0 {
		t.Fatalf("regular 3.77 Hz ECHO counted as dropouts: %+v", echo)
	}
	if !strings.Contains(session.Diagnosis.Headline, "TRIG measurement is not trustworthy") {
		t.Fatalf("diagnosis = %s", session.Diagnosis.Headline)
	}
}
