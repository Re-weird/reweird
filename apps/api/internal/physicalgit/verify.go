package physicalgit

import (
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/testplanner"
)

// VerifyInput bundles the already-resolved, already-observed evidence Verify
// compares against a target Physical Commit. Nothing here is fetched by
// Verify itself -- the caller resolves every field ahead of time (mirroring
// how createPhysicalCommit resolves its own evidence), which is what
// guarantees Verify never invokes Gemini or any other external service.
type VerifyInput struct {
	// CurrentProfile is the project's LIVE current ProjectProfile (Circuit
	// Map), not a commit snapshot -- Verify asks "does the live state match
	// the target", not "did some other commit match the target".
	CurrentProfile *domain.ProjectProfile
	// TargetMeasurement/CurrentMeasurement are the already-resolved
	// MeasurementWindow rows for the target commit and for the project's
	// most recently captured measurement, if any.
	TargetMeasurement  *domain.MeasurementWindow
	CurrentMeasurement *domain.MeasurementWindow
	// ObservationImage/ObservationVision are the current image/persisted
	// vision analysis used for visual verification. This app has no
	// standalone "verification photo" mechanism, so the caller supplies
	// whichever Physical Commit's image counts as the current observation
	// (by default, the project's most recent commit).
	ObservationImage  *domain.ProjectMedia
	ObservationVision *domain.PhysicalCommitVisionAnalysis
	TargetVision      *domain.PhysicalCommitVisionAnalysis
}

// Verify deterministically asks whether newly observed evidence supports
// having restored a project to target's captured state. It never mutates
// target and never invokes Gemini: electrical verification reuses
// testplanner.Verify's existing profile-limit comparison instead of a
// competing algorithm, and visual/semantic verification only ever reads
// already-captured/already-persisted evidence.
//
// Overall is derived deterministically from the five category results:
//   - any category NOT_SUPPORTED -> Overall NOT_SUPPORTED
//   - else at least one category SUPPORTED (and none NOT_SUPPORTED) ->
//     Overall SUPPORTED ("supported by available evidence" -- categories
//     that are INCONCLUSIVE/NOT_CAPTURED/UNAVAILABLE never block this)
//   - else -> Overall INCONCLUSIVE (nothing supports or contradicts it)
func Verify(target domain.PhysicalCommit, input VerifyInput) domain.PhysicalVerifyResult {
	result := domain.PhysicalVerifyResult{ProjectID: target.ProjectID, TargetCommit: target.ID}
	result.Components = verifyComponents(target.ProfileSnapshot, input.CurrentProfile)
	result.Circuit = verifyCircuit(target.ProfileSnapshot, input.CurrentProfile)
	result.Electrical = verifyElectrical(target, input)
	result.Visual = verifyVisual(target, input)
	result.Software = verifySoftware(target)

	categories := []domain.VerifyCategoryResult{result.Components, result.Circuit, result.Electrical, result.Visual, result.Software}
	anySupported, anyNotSupported := false, false
	for _, category := range categories {
		switch category.Status {
		case domain.VerifySupported:
			anySupported = true
		case domain.VerifyNotSupported:
			anyNotSupported = true
		}
	}
	switch {
	case anyNotSupported:
		result.Overall, result.Summary = domain.VerifyNotSupported, "Restoration is not supported by available evidence."
	case anySupported:
		result.Overall, result.Summary = domain.VerifySupported, "Restoration supported by available evidence."
	default:
		result.Overall, result.Summary = domain.VerifyInconclusive, "Not enough evidence is available to determine whether restoration is supported."
	}
	return result
}

func verifyComponents(target, current *domain.ProjectProfile) domain.VerifyCategoryResult {
	diff := componentsDiff(target, current)
	return mapEvidenceStateToVerify(diff.Status, len(diff.Changes), "component")
}

func verifyCircuit(target, current *domain.ProjectProfile) domain.VerifyCategoryResult {
	diff := circuitDiff(target, current)
	return mapEvidenceStateToVerify(diff.Status, len(diff.Changes), "circuit connection")
}

func mapEvidenceStateToVerify(status domain.EvidenceState, changeCount int, noun string) domain.VerifyCategoryResult {
	switch status {
	case domain.EvidenceNotCaptured:
		return domain.VerifyCategoryResult{Status: domain.VerifyNotCaptured}
	case domain.EvidenceUnavailable:
		return domain.VerifyCategoryResult{Status: domain.VerifyUnavailable, Detail: fmt.Sprintf("The target's or the current %s evidence could not be resolved for comparison.", noun)}
	case domain.EvidenceUnchanged:
		return domain.VerifyCategoryResult{Status: domain.VerifySupported, Detail: fmt.Sprintf("Current %s state matches the target commit.", noun)}
	default: // EvidenceChanged
		return domain.VerifyCategoryResult{Status: domain.VerifyNotSupported, Detail: fmt.Sprintf("%d %s difference(s) remain relative to the target commit.", changeCount, noun)}
	}
}

// verifyElectrical reuses testplanner.Verify's existing deterministic
// profile-limit comparison. The target commit's measurement is treated as
// the "before" (known-good reference) side and the current measurement as
// "after": testplanner.Verify's RESOLVED/IMPROVED/UNCHANGED outcomes all
// mean the current measurement is at least as good as the target's against
// applicable confirmed profile limits, which supports restoration; WORSE
// means it is not. No tolerance is invented here -- it is entirely
// testplanner's own existing severity comparison.
func verifyElectrical(target domain.PhysicalCommit, input VerifyInput) domain.VerifyCategoryResult {
	switch {
	case target.MeasurementID == nil && input.CurrentMeasurement == nil:
		return domain.VerifyCategoryResult{Status: domain.VerifyNotCaptured}
	case target.MeasurementID == nil || input.TargetMeasurement == nil:
		return domain.VerifyCategoryResult{Status: domain.VerifyUnavailable, Detail: "The target commit's measurement could not be resolved."}
	case input.CurrentMeasurement == nil:
		return domain.VerifyCategoryResult{Status: domain.VerifyUnavailable, Detail: "No current measurement has been captured yet."}
	case input.CurrentProfile == nil:
		return domain.VerifyCategoryResult{Status: domain.VerifyInconclusive, Detail: "No confirmed Project Profile is available to judge applicable limits."}
	}

	targets := make([]string, 0, len(input.TargetMeasurement.Analysis.Probes))
	for _, facts := range input.TargetMeasurement.Analysis.Probes {
		targets = append(targets, facts.Probe)
	}
	verification := testplanner.New().Verify(*input.CurrentProfile, targets, *input.TargetMeasurement, *input.CurrentMeasurement)
	switch verification.Status {
	case "RESOLVED", "IMPROVED", "UNCHANGED":
		return domain.VerifyCategoryResult{Status: domain.VerifySupported, Detail: verification.Summary, Changes: metricChangesToFieldChanges(verification.Changes)}
	case "WORSE":
		return domain.VerifyCategoryResult{Status: domain.VerifyNotSupported, Detail: verification.Summary, Changes: metricChangesToFieldChanges(verification.Changes)}
	default: // INCONCLUSIVE
		return domain.VerifyCategoryResult{Status: domain.VerifyInconclusive, Detail: verification.Summary}
	}
}

func metricChangesToFieldChanges(changes []domain.MetricChange) []domain.FieldChange {
	fields := make([]domain.FieldChange, 0, len(changes))
	for _, change := range changes {
		fields = append(fields, domain.FieldChange{Field: change.Probe + " " + change.Metric, Before: change.Before, After: change.After})
	}
	return fields
}

// verifyVisual is deliberately conservative: raw image bytes differing, or
// an AI-detected component count differing, is suggestive but never proof
// of a mismatch, so this category can never return NOT_SUPPORTED.
func verifyVisual(target domain.PhysicalCommit, input VerifyInput) domain.VerifyCategoryResult {
	raw := visualDiff(target.Image, input.ObservationImage)
	semantic := DiffVisionAnalyses(input.TargetVision, input.ObservationVision)

	switch raw.Status {
	case domain.EvidenceNotCaptured:
		if semantic.Status == domain.EvidenceNotCaptured {
			return domain.VerifyCategoryResult{Status: domain.VerifyNotCaptured}
		}
		return domain.VerifyCategoryResult{Status: domain.VerifyUnavailable, Detail: "No raw image evidence was captured on at least one side."}
	case domain.EvidenceAdded, domain.EvidenceRemoved:
		return domain.VerifyCategoryResult{Status: domain.VerifyUnavailable, Detail: "Only one of the target commit and the current observation has a captured image."}
	case domain.EvidenceChanged:
		return domain.VerifyCategoryResult{Status: domain.VerifyInconclusive, Detail: "The raw image differs; byte-level image difference alone cannot prove or disprove a hardware match."}
	}
	// raw.Status == EvidenceUnchanged from here.
	if semantic.Status == domain.EvidenceChanged {
		return domain.VerifyCategoryResult{Status: domain.VerifyInconclusive, Detail: "The raw image matches, but AI-detected components differ between the two analyses (unconfirmed interpretation)."}
	}
	return domain.VerifyCategoryResult{Status: domain.VerifySupported, Detail: "The current raw image matches the target commit's captured image."}
}

// verifySoftware never blocks restoration verification: Physical Git's
// software/GitHub integration is out of scope for this milestone, and a
// commit's software fields are always nil today. If they are ever
// populated by a later milestone, this compares them the same
// deterministic way softwareDiff already does -- never a Git checkout.
func verifySoftware(target domain.PhysicalCommit) domain.VerifyCategoryResult {
	if target.SoftwareProvider == nil && target.SoftwareRepository == nil && target.SoftwareRevision == nil {
		return domain.VerifyCategoryResult{Status: domain.VerifyNotCaptured}
	}
	return domain.VerifyCategoryResult{Status: domain.VerifyInconclusive, Detail: "Software/GitHub verification is not implemented in this milestone."}
}
