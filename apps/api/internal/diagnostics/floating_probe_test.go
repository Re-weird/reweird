package diagnostics

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/passport"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
)

func TestFloatingPhysicalTrigNeverBecomesBelievableKilohertz(t *testing.T) {
	profile := physicalfixture.Profile()
	analyzer := signalanalysis.New()
	var windows []domain.MeasurementWindow
	for i := 0; i < 10; i++ {
		frame := physicalfixture.Frame(uint64(i + 1))
		frame.Samples[1].PeriodsUS = []float64{1e6 / 3.79, 1e6 / 3.79, 1e6 / 3.79}
		frame.Samples[1].HighPulseWidthsUS = []float64{11.4, 11.4, 11.4, 11.4}
		analysis, err := analyzer.Analyze(frame, profile)
		if err != nil {
			t.Fatal(err)
		}
		windows = append(windows, domain.MeasurementWindow{ID: int64(i + 1), ProfileID: profile.ID, Source: "serial", DeviceID: frame.DeviceID, Sequence: frame.Sequence, IngestedAtMS: int64(1000 + i), Raw: frame, Analysis: analysis})
	}
	baseline, err := passport.LearnKnownGood(profile, windows, "")
	if err != nil {
		t.Fatal(err)
	}
	profile = passport.ApplyKnownGood(profile, domain.BaselinePhysical, &baseline)
	for _, kind := range []string{"unmatched", "paired glitches", "paired burst", "silent"} {
		t.Run(kind, func(t *testing.T) {
			frame := physicalfixture.Frame(50)
			sample := &frame.Samples[1]
			sample.RisingEdges = 19
			sample.FallingEdges = 0
			sample.EdgeCount = 19
			sample.PeriodsUS = []float64{26.388889, 26.388889, 26.388889}
			sample.HighPulseWidthsUS = nil
			sample.ActivityCounts = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 19}
			if kind == "paired glitches" || kind == "paired burst" {
				sample.FallingEdges = 19
				sample.EdgeCount = 38
				sample.HighPulseWidthsUS = []float64{.1, .1, .1}
				if kind == "paired burst" {
					sample.HighPulseWidthsUS = []float64{11.4, 11.4, 11.4}
				}
			}
			if kind == "silent" {
				sample.RisingEdges = 0
				sample.EdgeCount = 0
				sample.PeriodsUS = nil
				sample.ActivityCounts = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			}
			// Secondary timing/analog differences must remain visible, not become focus.
			frame.Samples[2].PeriodsUS = []float64{240000, 240000, 240000}
			frame.Samples[3].HighPulseWidthsUS = []float64{3000, 3000, 3000}
			frame.Samples[4].AnalogMV = []float64{2000, 2001, 2000}
			rawJSON, _ := json.Marshal(frame)
			session, err := NewEngine(analyzer).AnalyzeEnvelope(context.Background(), profile, domain.StageDiagnose, "serial", frame, nil)
			if err != nil {
				t.Fatal(err)
			}
			facts, _ := session.Analysis.Probe("P2")
			if kind != "silent" && !facts.CaptureUnreliable {
				t.Fatal("floating capture was trusted")
			}
			if facts.FrequencyHz != nil || facts.AveragePulseWidthUS != nil || facts.MinimumPulseWidthUS != nil || facts.MaximumPulseWidthUS != nil || facts.DutyCyclePercent != nil || facts.JitterUS != nil {
				t.Fatalf("invalid timing escaped: %+v", facts)
			}
			if kind != "silent" && facts.DropoutEvents != 0 {
				t.Fatal("glitch periods became dropout counts")
			}
			if !strings.Contains(session.Diagnosis.Headline, "TRIG activity missing or unreliable compared with Known Good") || session.Diagnosis.Confidence > .5 {
				t.Fatalf("bad diagnosis %+v", session.Diagnosis)
			}
			if session.Evidence.RuleResults[0].Probe != "P2" {
				t.Fatal("secondary evidence obscured P2")
			}
			if !hasRuleForProbe(session.Evidence.RuleResults, "frequency-outside-specification", "P3") {
				t.Fatal("secondary evidence erased")
			}
			for _, reading := range session.Probes {
				if reading.Probe == "P2" && kind != "silent" && reading.Value != nil {
					t.Fatal("invalid capture displayed as reading")
				}
			}
			rawAfter, _ := json.Marshal(session.RawTelemetry)
			if !reflect.DeepEqual(rawJSON, rawAfter) || session.TelemetryMode != "serial" {
				t.Fatal("raw evidence/provenance changed")
			}
			recommendation := testplanner.New().Recommend(profile, session.Analysis, session.ID)
			if recommendation.TestType != domain.TestRemeasure || len(recommendation.TargetProbes) != 1 || recommendation.TargetProbes[0] != "P2" || !strings.Contains(recommendation.Reason, "TRIG") {
				t.Fatalf("wrong next test %+v", recommendation)
			}
			if _, err := testplanner.New().Plan(profile, recommendation); err != nil {
				t.Fatal(err)
			}
		})
	}
}
