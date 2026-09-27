package httpapi

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/physicalfixture"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
)

func TestPhysicalRemeasureReconcilesPersistedAndNewAfter(t *testing.T) {
	repository, err := store.Open(filepath.Join(t.TempDir(), "results.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	profile := physicalfixture.Profile()
	hz, width := 3.79, 11.4
	profile.Probes[1].Baseline = &domain.TrustedBaseline{Status: domain.BaselineUserConfirmedHealthy, WindowCount: 10, FrequencyHz: &hz, MinFrequencyHz: &hz, MaxFrequencyHz: &hz, PulseWidthUS: &width, MinPulseWidthUS: &width, MaxPulseWidthUS: &width, FrequencyTolerancePct: 10}
	if err := repository.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	makeWindow := func(id int64, sequence uint64, absent bool) domain.MeasurementWindow {
		frame := physicalfixture.Frame(sequence)
		if absent {
			zero := 0
			frame.Samples[1] = domain.TelemetrySample{Probe: "P2", Mode: domain.ProbeModePulse, State: &zero, MaxGapUS: 1000000, ActivityCounts: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
		}
		analysis, err := signalanalysis.New().Analyze(frame, profile)
		if err != nil {
			t.Fatal(err)
		}
		return domain.MeasurementWindow{ID: id, ProfileID: profile.ID, DeviceID: frame.DeviceID, Source: "serial", Sequence: sequence, Raw: frame, Analysis: analysis}
	}
	before, during, after := makeWindow(3378, 1, true), makeWindow(3488, 2, true), makeWindow(3538, 3, false)
	planner := testplanner.New()
	plan, err := planner.Plan(profile, domain.TestRecommendation{ID: "test-reconcile", SessionID: "session", TestType: domain.TestRemeasure, TargetProbes: []string{"P2"}, Reason: "Restore passive monitoring", DurationSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	stale := planner.Evaluate(profile, plan, before, during)
	if stale.Result != "UNCHANGED" {
		t.Fatal(stale)
	}
	oldVerify := planner.Verify(profile, []string{"P2"}, during, after)
	if oldVerify.Status != "RESOLVED" {
		t.Fatal(oldVerify)
	}
	workflow := domain.DiagnosticWorkflow{ID: plan.ID, ProfileID: profile.ID, ProfileVersion: profile.Version, ProfileSnapshot: &profile, Plan: plan, Status: domain.TestResolved, Baseline: &before, During: &during, After: &after, Result: &stale, Verification: &oldVerify}
	if err := repository.SaveTestWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	app := NewApp(diagnostics.NewEngine(signalanalysis.New()), repository, fixedSerialSource{after.Raw}, profile.ID, ProjectServices{}, false)
	check := func(w domain.DiagnosticWorkflow) {
		t.Helper()
		if w.Result == nil || w.Verification == nil || w.Result.Result != "RESOLVED" || w.Verification.Status != "RESOLVED" {
			t.Fatalf("contradictory results %+v", w)
		}
		if w.Verification.BeforeWindowID != before.ID || w.Verification.AfterWindowID != w.After.ID {
			t.Fatal("VERIFY used wrong captures")
		}
		if w.Result.DerivedMetrics["before_window_id"] != float64(before.ID) || w.Result.DerivedMetrics["after_window_id"] != float64(w.After.ID) || w.Result.DerivedMetrics["during_window_id"] != float64(during.ID) {
			t.Fatal(w.Result.DerivedMetrics)
		}
		if w.Result.DerivedMetrics["during_result"] != "UNCHANGED" || w.Result.DerivedMetrics["capture_source"] != "serial" {
			t.Fatal("intermediate evidence or provenance lost")
		}
		found := false
		for _, change := range w.Verification.Changes {
			if change.Probe == "P2" && change.Metric == "dropout_rate_per_minute" && change.Before == float64(180) && change.After == float64(0) {
				found = true
			}
		}
		if !found {
			t.Fatal(w.Verification.Changes)
		}
	}
	for _, path := range []string{"/api/v1/tests/" + workflow.ID, "/api/v1/tests/current"} {
		var read domain.DiagnosticWorkflow
		decodeBody(t, doJSON(t, app, http.MethodGet, path, nil), &read)
		check(read)
	}
	// A new After capture also reconciles before it is persisted.
	workflow.After = nil
	workflow.Verification = nil
	workflow.Status = domain.TestCompleted
	if err := repository.SaveTestWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	var updated domain.DiagnosticWorkflow
	decodeBody(t, doJSON(t, app, http.MethodPost, "/api/v1/tests/"+workflow.ID+"/remeasure", nil), &updated)
	check(updated)
	stored, err := repository.GetTestWorkflow(workflow.ID)
	if err != nil {
		t.Fatal(err)
	}
	check(*stored)
	// Cross-source evidence must not claim successful physical recovery.
	updated.After.Source = "simulator"
	planner.ReconcileRemeasure(&updated)
	if updated.Result.Result != "INCONCLUSIVE" || updated.Verification.Status != "INCONCLUSIVE" {
		t.Fatal("mixed provenance accepted")
	}
}
