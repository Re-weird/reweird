package testplanner

import (
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func ptr(value float64) *float64 { return &value }

func genericProfile() domain.ProjectProfile {
	return domain.ProjectProfile{ID: "generic-fan", Confirmed: true, Probes: []domain.ProbeConfiguration{
		{Probe: "P6", Role: "FAN_TACH", Mode: domain.ProbeModePulse, Expected: domain.ExpectedSignal{Required: true, Stable: true, MinFrequencyHz: ptr(90), MaxFrequencyHz: ptr(110), MaxDropouts: 0}, Baseline: &domain.TrustedBaseline{Status: domain.BaselineKnownGoodCapture, FrequencyHz: ptr(100), FrequencyTolerancePct: 10}},
		{Probe: "P5", Role: "SUPPLY", Mode: domain.ProbeModeAnalog, IsPowerRail: true, Expected: domain.ExpectedSignal{Required: true, Stable: true, MinVoltage: ptr(3.1), MaxVoltage: ptr(3.5)}},
	}}
}

func TestPlanSupportsAllGenericTypesAndPatchLock(t *testing.T) {
	profile := genericProfile()
	cases := []struct {
		kind    domain.TestType
		targets []string
	}{
		{domain.TestMovementCorrelation, []string{"P6"}}, {domain.TestPowerRailStability, []string{"P5"}},
		{domain.TestSimultaneousDropout, []string{"P5", "P6"}}, {domain.TestSignalActivity, []string{"P6"}},
		{domain.TestBaselineComparison, []string{"P6"}}, {domain.TestFrequencyTiming, []string{"P6"}}, {domain.TestRemeasure, []string{"P6"}},
	}
	for _, item := range cases {
		t.Run(string(item.kind), func(t *testing.T) {
			plan, err := New().Plan(profile, domain.TestRecommendation{ID: "rec", SessionID: "session", TestType: item.kind, TargetProbes: item.targets, Reason: "A measured anomaly needs a test.", DurationSeconds: 12})
			if err != nil || plan.WindowMS != 12_000 || len(plan.Instructions) == 0 || !strings.Contains(strings.Join(plan.Monitoring, " "), item.targets[0]) {
				t.Fatalf("plan = %+v, err = %v", plan, err)
			}
		})
	}
	plan, err := New().Plan(profile, domain.TestRecommendation{ID: "rec", SessionID: "session", TestType: domain.TestRemeasure, TargetProbes: []string{"P6"}, Reason: "Need remeasurement", RequiresPatch: true})
	if err != nil || !plan.RequiresPatch || plan.Unavailable == "" {
		t.Fatalf("patch plan = %+v, err=%v", plan, err)
	}
	if _, err := New().Plan(profile, domain.TestRecommendation{ID: "rec", SessionID: "session", TestType: domain.TestRemeasure, TargetProbes: []string{"P3"}, Reason: "Invalid"}); err == nil {
		t.Fatal("unassigned probe accepted")
	}
}

func TestGenericVerifyDistinguishesOutcomes(t *testing.T) {
	profile := genericProfile()
	window := func(id int64, freq float64, dropouts int) domain.MeasurementWindow {
		return domain.MeasurementWindow{ID: id, ProfileID: profile.ID, Analysis: domain.AnalysisResult{ProfileID: profile.ID, WindowMS: 60_000, Probes: []domain.DerivedFacts{{Probe: "P6", FrequencyHz: ptr(freq), DropoutEvents: dropouts, Stable: dropouts == 0 && freq >= 90 && freq <= 110}}}}
	}
	before := window(1, 70, 4)
	cases := []struct {
		name  string
		after domain.MeasurementWindow
		want  string
	}{
		{"resolved", window(2, 100, 0), "RESOLVED"},
		{"improved", window(2, 85, 1), "IMPROVED"},
		{"unchanged", window(2, 70, 4), "UNCHANGED"},
		{"worse", window(2, 50, 6), "WORSE"},
		{"missing second window", domain.MeasurementWindow{}, "INCONCLUSIVE"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			result := New().Verify(profile, []string{"P6"}, before, item.after)
			if result.Status != item.want {
				t.Fatalf("result = %+v, want %s", result, item.want)
			}
			if item.want == "RESOLVED" && (len(result.Changes) == 0 || len(result.RemainingIssues) > 0) {
				t.Fatalf("resolved details = %+v", result)
			}
		})
	}
	unknown := genericProfile()
	unknown.Probes[0].Expected = domain.ExpectedSignal{}
	unknown.Probes[0].Baseline.Status = domain.BaselineUnknown
	if result := New().Verify(unknown, []string{"P6"}, before, window(2, 100, 0)); result.Status != "INCONCLUSIVE" {
		t.Fatalf("unknown baseline became authoritative: %+v", result)
	}
}

func TestMovementCorrelationDoesNotClaimPhysicalCause(t *testing.T) {
	profile := genericProfile()
	recommendation := domain.TestRecommendation{ID: "movement", SessionID: "session", TestType: domain.TestMovementCorrelation, TargetProbes: []string{"P6"}, Reason: "dropouts", DurationSeconds: 10}
	plan, err := New().Plan(profile, recommendation)
	if err != nil {
		t.Fatal(err)
	}
	window := func(id int64, dropouts int) domain.MeasurementWindow {
		return domain.MeasurementWindow{ID: id, Analysis: domain.AnalysisResult{ProfileID: profile.ID, WindowMS: 60_000, Probes: []domain.DerivedFacts{{Probe: "P6", DropoutEvents: dropouts}}}}
	}
	result := New().Evaluate(profile, plan, window(1, 2), window(2, 6))
	if result.Result != "POSITIVE_CORRELATION" || !strings.Contains(result.Interpretation, "not a specific physical cause") {
		t.Fatalf("result = %+v", result)
	}
}
