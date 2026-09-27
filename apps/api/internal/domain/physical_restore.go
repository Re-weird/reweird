package domain

// RestoreStatus is the deterministic outcome of a single Restore checklist
// item, or of an entire Restore section. It is distinct from EvidenceState:
// EvidenceState answers "did this evidence category change between two
// commits", while RestoreStatus answers "what should the user do about it
// right now". NOT_CAPTURED and UNAVAILABLE keep the same meaning as
// EvidenceState's states of the same name.
type RestoreStatus string

const (
	// RestoreMatch means the current/reference state already matches the
	// target commit for this item -- nothing to do.
	RestoreMatch RestoreStatus = "MATCH"
	// RestoreActionRequired means a concrete, deterministic structural
	// action would bring this item back to the target's captured state.
	RestoreActionRequired RestoreStatus = "ACTION_REQUIRED"
	// RestoreNotCaptured means neither the target nor the reference commit
	// captured this evidence category at all.
	RestoreNotCaptured RestoreStatus = "NOT_CAPTURED"
	// RestoreUnavailable means evidence exists on at least one side but
	// cannot be meaningfully compared (e.g. no reference commit was
	// selected, or a referenced row failed to resolve).
	RestoreUnavailable RestoreStatus = "UNAVAILABLE"
	// RestoreVerifyRequired means this category cannot be satisfied by a
	// structural action alone -- it must be re-measured/re-observed after
	// structural restoration to know whether it actually matches again.
	RestoreVerifyRequired RestoreStatus = "VERIFY_REQUIRED"
)

// RestoreActionCategory groups a RestoreAction the same way Physical Diff
// groups its sections.
type RestoreActionCategory string

const (
	RestoreCategoryComponents RestoreActionCategory = "COMPONENTS"
	RestoreCategoryCircuit    RestoreActionCategory = "CIRCUIT"
	RestoreCategoryElectrical RestoreActionCategory = "ELECTRICAL"
	RestoreCategoryVisual     RestoreActionCategory = "VISUAL"
	RestoreCategorySoftware   RestoreActionCategory = "SOFTWARE"
)

// RestoreAction is a single deterministic checklist line. TargetValue and
// CurrentValue hold the actual stored values being compared (as display
// strings) -- never a generated causal sentence. Description is a plain,
// deterministic instruction template ("Restore ECHO connection to GPIO18."),
// not free text from an LLM.
type RestoreAction struct {
	Category     RestoreActionCategory `json:"category"`
	Status       RestoreStatus         `json:"status"`
	Title        string                `json:"title"`
	Description  string                `json:"description,omitempty"`
	TargetValue  string                `json:"target_value,omitempty"`
	CurrentValue string                `json:"current_value,omitempty"`
	// AIInterpreted marks an action derived only from a persisted Gemini
	// Vision analysis rather than structured, measured evidence -- the
	// frontend must never present this the same way as a raw-evidence
	// action.
	AIInterpreted bool `json:"ai_interpreted,omitempty"`
}

// RestoreSection is one category's overall status plus its checklist items.
// Status is the deterministic aggregate of Actions: ACTION_REQUIRED if any
// action needs one, else VERIFY_REQUIRED if any needs re-measurement, else
// NOT_CAPTURED/UNAVAILABLE if evidence was missing/incomparable, else MATCH.
type RestoreSection struct {
	Status  RestoreStatus   `json:"status"`
	Actions []RestoreAction `json:"actions,omitempty"`
}

// PhysicalRestorePlan is a deterministic, structured restoration checklist
// for returning a project's OBSERVABLE state to a target Physical Commit.
// It never physically modifies hardware and never invokes Gemini -- it is
// computed on demand from already-captured evidence, the same way
// PhysicalCommitDiff is, and reuses the same comparison building blocks.
type PhysicalRestorePlan struct {
	ProjectID    string `json:"project_id"`
	TargetCommit string `json:"target_commit"`
	// SourceCommit is the current/reference commit being compared against
	// the target. Empty when no reference commit was available/selected --
	// HasSource distinguishes "empty because none exists" from a real id.
	SourceCommit string `json:"source_commit,omitempty"`
	HasSource    bool   `json:"has_source"`

	Components RestoreSection `json:"components"`
	Circuit    RestoreSection `json:"circuit"`
	Electrical RestoreSection `json:"electrical"`
	Visual     RestoreSection `json:"visual"`
	Software   RestoreSection `json:"software"`
}
