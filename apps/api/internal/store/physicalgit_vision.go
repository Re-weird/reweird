package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var _ domain.PhysicalCommitVisionAnalysisRepository = (*SQLiteStore)(nil)

// SavePhysicalCommitVisionAnalysis upserts by PhysicalCommitID -- a single
// keyed upsert is already atomic at the SQL level, so no mutex is needed
// here (unlike physical_commits' sequence counter).
func (store *SQLiteStore) SavePhysicalCommitVisionAnalysis(record domain.PhysicalCommitVisionAnalysis) (domain.PhysicalCommitVisionAnalysis, error) {
	if record.PhysicalCommitID == "" || record.ProjectID == "" {
		return domain.PhysicalCommitVisionAnalysis{}, fmt.Errorf("incomplete physical commit vision analysis")
	}
	record.CreatedAtMS = time.Now().UTC().UnixMilli()
	payload, err := json.Marshal(record)
	if err != nil {
		return domain.PhysicalCommitVisionAnalysis{}, err
	}
	if _, err := store.db.Exec(`
		INSERT INTO physical_commit_vision_analyses (physical_commit_id, project_id, created_at_ms, payload)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(physical_commit_id) DO UPDATE SET
			project_id = excluded.project_id,
			created_at_ms = excluded.created_at_ms,
			payload = excluded.payload
	`, record.PhysicalCommitID, record.ProjectID, record.CreatedAtMS, string(payload)); err != nil {
		return domain.PhysicalCommitVisionAnalysis{}, err
	}
	return record, nil
}

// GetPhysicalCommitVisionAnalysis filters by both physical_commit_id and
// project_id so a record from a different project 404s the same way a
// missing one does, never leaking cross-project existence.
func (store *SQLiteStore) GetPhysicalCommitVisionAnalysis(projectID, physicalCommitID string) (*domain.PhysicalCommitVisionAnalysis, error) {
	var payload string
	err := store.db.QueryRow(`
		SELECT payload FROM physical_commit_vision_analyses WHERE physical_commit_id = ? AND project_id = ?
	`, physicalCommitID, projectID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record domain.PhysicalCommitVisionAnalysis
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, err
	}
	return &record, nil
}
