package domain

import "encoding/json"

// PhysicalCommit records what ReWeird knew about a hardware project's
// physical/electrical state at a specific moment. History is linear: there
// are no parent ids, branches, or merges.
//
// Mutable "current state" (ProjectProfile, KnownGoodBaseline) is either
// copied by value (ProfileSnapshot) or referenced by an already-immutable
// historical id (PassportBaselineIDs, MeasurementID) so that later edits to
// the live project never retroactively change what a past commit says.
type PhysicalCommit struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Sequence    int    `json:"sequence"`
	DisplayID   string `json:"display_id"`
	Note        string `json:"note,omitempty"`
	CreatedAtMS int64  `json:"created_at_ms"`

	// Image is the raw camera/uploaded photo reference for this commit, if any.
	Image *ProjectMedia `json:"image,omitempty"`

	// ProfileSnapshot is a full copy of the ProjectProfile at commit time:
	// component state and the Circuit Map (Components/Connections/Probes).
	ProfileSnapshot *ProjectProfile `json:"profile_snapshot,omitempty"`

	// PassportBaselineIDs reference the KnownGoodBaseline rows (already
	// immutable) that were the active baseline at commit time. Only the
	// physical baseline is ever auto-attached here -- a simulated baseline
	// belongs to the synthetic/game-demo workflow, never to real Physical
	// Git history. Not copied, since these rows never change once saved.
	PassportBaselineIDs []int64 `json:"passport_baseline_ids,omitempty"`

	// MeasurementID references the most recent valid MeasurementWindow for
	// this project at commit time. Nil means no measurement was available;
	// it is never fabricated.
	MeasurementID *int64 `json:"measurement_id,omitempty"`

	// Software state is prepared for later GitHub integration. All fields
	// stay nil in V1.
	SoftwareProvider   *string          `json:"software_provider,omitempty"`
	SoftwareRepository *string          `json:"software_repository,omitempty"`
	SoftwareRevision   *string          `json:"software_revision,omitempty"`
	SoftwareAnalysis   *json.RawMessage `json:"software_analysis,omitempty"`
}

// PhysicalCommitRepository is implemented by the persistence layer. The
// caller pre-generates ID; SavePhysicalCommit allocates the project-scoped
// Sequence/DisplayID and CreatedAtMS.
type PhysicalCommitRepository interface {
	SavePhysicalCommit(commit PhysicalCommit) (PhysicalCommit, error)
	GetPhysicalCommit(projectID, id string) (*PhysicalCommit, error)
	ListPhysicalCommits(projectID string) ([]PhysicalCommit, error)
}

// PhysicalCommitDetail is the read-time-enriched view returned by
// GET .../physical-commits/:commitId/detail. It resolves MeasurementID into
// the full MeasurementWindow for display; the stored PhysicalCommit itself
// is never mutated to hold this, and a resolution failure never fails the
// request -- Measurement is simply omitted, never fabricated.
type PhysicalCommitDetail struct {
	Commit      PhysicalCommit     `json:"commit"`
	Measurement *MeasurementWindow `json:"measurement,omitempty"`
}
