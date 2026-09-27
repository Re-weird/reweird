package domain

// PhysicalCommitVisionAnalysis is a persisted AI interpretation of a
// Physical Commit's raw image -- never the raw image itself, and never
// merged back into ProjectProfile/Circuit Map/Device Passport/measurements.
// One record per commit: a successful re-analysis replaces the prior one.
// Only a genuinely successful Gemini result (VisionAnalysis.Status ==
// "VISION_COMPLETE") is ever persisted here -- a skipped or failed attempt
// is neither evidence nor interpretation, so it is never stored.
type PhysicalCommitVisionAnalysis struct {
	ID               string         `json:"id"`
	ProjectID        string         `json:"project_id"`
	PhysicalCommitID string         `json:"physical_commit_id"`
	Provider         string         `json:"provider"`
	Analysis         VisionAnalysis `json:"analysis"`
	CreatedAtMS      int64          `json:"created_at_ms"`
}

// PhysicalCommitVisionAnalysisRepository is implemented by the persistence
// layer. SavePhysicalCommitVisionAnalysis upserts by PhysicalCommitID.
type PhysicalCommitVisionAnalysisRepository interface {
	SavePhysicalCommitVisionAnalysis(record PhysicalCommitVisionAnalysis) (PhysicalCommitVisionAnalysis, error)
	GetPhysicalCommitVisionAnalysis(projectID, physicalCommitID string) (*PhysicalCommitVisionAnalysis, error)
}
