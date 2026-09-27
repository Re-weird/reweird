package domain

// VerifyStatus is the deterministic outcome of comparing newly observed
// evidence against a target Physical Commit for one evidence category, or
// the overall result derived from those categories. It is evidence-based:
// a category is never SUPPORTED just because the user says restoration is
// done.
type VerifyStatus string

const (
	// VerifySupported means the newly observed evidence is consistent with
	// the target commit's captured state for this category.
	VerifySupported VerifyStatus = "SUPPORTED"
	// VerifyNotSupported means the newly observed evidence contradicts the
	// target commit's captured state for this category.
	VerifyNotSupported VerifyStatus = "NOT_SUPPORTED"
	// VerifyInconclusive means evidence exists on both sides but cannot be
	// judged one way or the other with the available deterministic rules
	// (e.g. a confirmed profile has no applicable limits for the probes
	// involved).
	VerifyInconclusive VerifyStatus = "INCONCLUSIVE"
	// VerifyNotCaptured means neither the target commit nor the new
	// observation captured this evidence category at all.
	VerifyNotCaptured VerifyStatus = "NOT_CAPTURED"
	// VerifyUnavailable means evidence exists on at least one side but a
	// valid comparison could not be made (e.g. a referenced row failed to
	// resolve, or no new observation of this category was ever requested).
	VerifyUnavailable VerifyStatus = "UNAVAILABLE"
)

// VerifyCategoryResult is one evidence category's verification outcome.
// Detail is a short, deterministic, factual explanation -- never a causal
// claim ("X caused Y").
type VerifyCategoryResult struct {
	Status  VerifyStatus  `json:"status"`
	Detail  string        `json:"detail,omitempty"`
	Changes []FieldChange `json:"changes,omitempty"`
}

// PhysicalVerifyResult is the outcome of asking whether newly observed
// hardware state supports having restored a project to a target Physical
// Commit. It never mutates the target commit, and it never invokes Gemini:
// electrical verification reuses testplanner.Verify's deterministic profile-
// limit comparison, and visual/semantic-visual verification only ever reads
// already-captured/already-persisted evidence.
type PhysicalVerifyResult struct {
	ProjectID    string `json:"project_id"`
	TargetCommit string `json:"target_commit"`

	Components VerifyCategoryResult `json:"components"`
	Circuit    VerifyCategoryResult `json:"circuit"`
	Electrical VerifyCategoryResult `json:"electrical"`
	Visual     VerifyCategoryResult `json:"visual"`
	Software   VerifyCategoryResult `json:"software"`

	// Overall is deterministically derived from the category results: never
	// a separate judgment call. See physicalgit.Verify's doc comment for
	// the exact derivation rule.
	Overall VerifyStatus `json:"overall"`
	Summary string       `json:"summary"`
}
