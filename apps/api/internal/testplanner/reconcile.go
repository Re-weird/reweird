package testplanner

import "github.com/re-weird/reweird/apps/api/internal/domain"

// ReconcileRemeasure recomputes recovery, not a UI label. Before/After and the
// frozen profile (including applicable Known Good) are authoritative for both
// independent evaluations. During remains intermediate evidence, not a repair
// outcome. Other test types retain their different test-vs-recovery criteria.
func (planner *Planner) ReconcileRemeasure(w *domain.DiagnosticWorkflow) {
	if w == nil || w.Plan.Recommendation.TestType != domain.TestRemeasure || w.Baseline == nil || w.During == nil || w.After == nil || w.ProfileSnapshot == nil {
		return
	}
	if w.Status == domain.TestCancelled || w.Status == domain.TestLocked || w.Status == domain.TestFailed {
		return // Reading historical evidence must not revive a stopped workflow.
	}
	profile := *w.ProfileSnapshot
	before, during, after := *w.Baseline, *w.During, *w.After
	valid := len(w.Plan.Recommendation.TargetProbes) > 0 && before.ID > 0 && during.ID > 0 && after.ID > 0 && before.ID != during.ID && before.ID != after.ID && during.ID != after.ID && profile.ID == w.ProfileID && profile.Version == w.ProfileVersion
	for _, capture := range []domain.MeasurementWindow{before, during, after} {
		valid = valid && capture.Source != "" && capture.Source == before.Source && capture.DeviceID != "" && capture.DeviceID == before.DeviceID && capture.ProfileID == profile.ID && capture.Analysis.ProfileID == profile.ID && (capture.Analysis.ProfileVersion == 0 || capture.Analysis.ProfileVersion == profile.Version)
	}
	result := domain.TestResult{TestID: w.Plan.ID, TestType: domain.TestRemeasure, TargetProbes: w.Plan.Recommendation.TargetProbes, Result: "INCONCLUSIVE", Interpretation: "Capture identity, source, device or profile revision does not match this workflow.", DerivedMetrics: map[string]any{}}
	verification := domain.VerificationResult{Status: "INCONCLUSIVE", Summary: result.Interpretation, BeforeWindowID: before.ID, AfterWindowID: after.ID}
	if valid {
		result = planner.Evaluate(profile, w.Plan, before, after)
		verification = planner.Verify(profile, w.Plan.Recommendation.TargetProbes, before, after)
		intermediate := planner.Evaluate(profile, w.Plan, before, during)
		result.DerivedMetrics["during_result"] = intermediate.Result
		result.DerivedMetrics["during_interpretation"] = intermediate.Interpretation
	}
	result.DerivedMetrics["before_window_id"] = before.ID
	result.DerivedMetrics["during_window_id"] = during.ID
	result.DerivedMetrics["after_window_id"] = after.ID
	result.DerivedMetrics["test_window_id"] = after.ID
	result.DerivedMetrics["evaluation_phase"] = "BEFORE_AFTER_RECOVERY"
	result.DerivedMetrics["criteria_profile_version"] = profile.Version
	result.DerivedMetrics["capture_source"] = before.Source
	if valid && before.Source == "serial" {
		result.EvidenceProvenance = append(result.EvidenceProvenance, domain.Provenance(domain.ProvenanceRealSerial))
	}
	w.Result, w.Verification = &result, &verification
	switch verification.Status {
	case "RESOLVED":
		w.Status = domain.TestResolved
	case "INCONCLUSIVE":
		w.Status = domain.TestInconclusive
	default:
		w.Status = domain.TestUnresolved
	}
}
