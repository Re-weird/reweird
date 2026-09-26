package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var _ domain.TestWorkflowRepository = (*SQLiteStore)(nil)

func (store *SQLiteStore) SaveTestWorkflow(workflow domain.DiagnosticWorkflow) error {
	if workflow.ID == "" || workflow.ProfileID == "" {
		return fmt.Errorf("test workflow id and profile id are required")
	}
	payload, err := json.Marshal(workflow)
	if err != nil {
		return err
	}
	_, err = store.db.Exec(`
		INSERT INTO test_workflows (id, profile_id, status, payload, created_at_ms, updated_at_ms)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status, payload = excluded.payload, updated_at_ms = excluded.updated_at_ms
	`, workflow.ID, workflow.ProfileID, workflow.Status, string(payload), workflow.CreatedAtMS, workflow.UpdatedAtMS)
	return err
}

func (store *SQLiteStore) GetTestWorkflow(id string) (*domain.DiagnosticWorkflow, error) {
	return store.readTestWorkflow("SELECT payload FROM test_workflows WHERE id = ?", id)
}

func (store *SQLiteStore) LatestTestWorkflow(profileID string) (*domain.DiagnosticWorkflow, error) {
	return store.readTestWorkflow("SELECT payload FROM test_workflows WHERE profile_id = ? ORDER BY updated_at_ms DESC, id DESC LIMIT 1", profileID)
}

// ListTestWorkflows pages over the existing persisted test records; it does
// not create a second session store. Callers can filter the bounded page.
func (store *SQLiteStore) ListTestWorkflows(limit, offset int) ([]domain.DiagnosticWorkflow, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := store.db.Query("SELECT payload FROM test_workflows ORDER BY created_at_ms DESC, id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.DiagnosticWorkflow, 0, limit)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var workflow domain.DiagnosticWorkflow
		if err := json.Unmarshal([]byte(payload), &workflow); err != nil {
			return nil, err
		}
		items = append(items, workflow)
	}
	return items, rows.Err()
}

func (store *SQLiteStore) readTestWorkflow(query, value string) (*domain.DiagnosticWorkflow, error) {
	var payload string
	err := store.db.QueryRow(query, value).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var workflow domain.DiagnosticWorkflow
	if err := json.Unmarshal([]byte(payload), &workflow); err != nil {
		return nil, err
	}
	return &workflow, nil
}
