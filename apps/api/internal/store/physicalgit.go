package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var _ domain.PhysicalCommitRepository = (*SQLiteStore)(nil)

func physicalCommitDisplayID(sequence int) string {
	return fmt.Sprintf("HW-%03d", sequence)
}

// SavePhysicalCommit assigns the next project-scoped sequence number and
// persists the commit. The mutex is the real serialization mechanism here
// (this table has a single writer path); a transaction would add nothing
// beyond what it already guarantees.
func (store *SQLiteStore) SavePhysicalCommit(commit domain.PhysicalCommit) (domain.PhysicalCommit, error) {
	if commit.ID == "" || commit.ProjectID == "" {
		return domain.PhysicalCommit{}, fmt.Errorf("incomplete physical commit")
	}
	store.physicalCommitMu.Lock()
	defer store.physicalCommitMu.Unlock()

	var sequence int
	if err := store.db.QueryRow(`SELECT COALESCE(MAX(sequence), 0) + 1 FROM physical_commits WHERE project_id = ?`, commit.ProjectID).Scan(&sequence); err != nil {
		return domain.PhysicalCommit{}, err
	}
	commit.Sequence = sequence
	commit.DisplayID = physicalCommitDisplayID(sequence)
	commit.CreatedAtMS = time.Now().UTC().UnixMilli()

	payload, err := json.Marshal(commit)
	if err != nil {
		return domain.PhysicalCommit{}, err
	}
	if _, err := store.db.Exec(`
		INSERT INTO physical_commits (id, project_id, sequence, created_at_ms, payload)
		VALUES (?, ?, ?, ?, ?)
	`, commit.ID, commit.ProjectID, commit.Sequence, commit.CreatedAtMS, string(payload)); err != nil {
		return domain.PhysicalCommit{}, err
	}
	return commit, nil
}

// GetPhysicalCommit filters by both id and project_id so a commit that
// belongs to a different project 404s the same way a missing id does, never
// leaking cross-project existence.
func (store *SQLiteStore) GetPhysicalCommit(projectID, id string) (*domain.PhysicalCommit, error) {
	var payload string
	err := store.db.QueryRow(`SELECT payload FROM physical_commits WHERE id = ? AND project_id = ?`, id, projectID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var commit domain.PhysicalCommit
	if err := json.Unmarshal([]byte(payload), &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}

func (store *SQLiteStore) ListPhysicalCommits(projectID string) ([]domain.PhysicalCommit, error) {
	rows, err := store.db.Query(`SELECT payload FROM physical_commits WHERE project_id = ? ORDER BY sequence DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PhysicalCommit, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var commit domain.PhysicalCommit
		if err := json.Unmarshal([]byte(payload), &commit); err != nil {
			return nil, err
		}
		items = append(items, commit)
	}
	return items, rows.Err()
}
