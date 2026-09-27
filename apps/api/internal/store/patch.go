package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/re-weird/reweird/apps/api/internal/patchcontrol"
)

func (s *SQLiteStore) SavePatchAction(a patchcontrol.Action) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO patch_actions(id,profile_id,payload) VALUES(?,?,?)
 ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, a.ID, a.Parameters.ProfileID, string(b))
	return err
}
func (s *SQLiteStore) GetPatchAction(id string) (*patchcontrol.Action, error) {
	var b string
	err := s.db.QueryRow("SELECT payload FROM patch_actions WHERE id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var a patchcontrol.Action
	err = json.Unmarshal([]byte(b), &a)
	return &a, err
}
func (s *SQLiteStore) ListPatchActions(profile string) ([]patchcontrol.Action, error) {
	rows, err := s.db.Query("SELECT payload FROM patch_actions WHERE profile_id=? ORDER BY rowid DESC LIMIT 100", profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []patchcontrol.Action{}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var a patchcontrol.Action
		if err := json.Unmarshal([]byte(b), &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *SQLiteStore) InterruptPatchActions(now int64) error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS patch_actions(id TEXT PRIMARY KEY,profile_id TEXT NOT NULL,payload TEXT NOT NULL)`); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT payload FROM patch_actions")
	if err != nil {
		return err
	}
	pending := []patchcontrol.Action{}
	for rows.Next() {
		var b string
		if err := rows.Scan(&b); err != nil {
			rows.Close()
			return err
		}
		var a patchcontrol.Action
		if err := json.Unmarshal([]byte(b), &a); err != nil {
			rows.Close()
			return err
		}
		switch a.State {
		case "LOCKED", "ABORTED", "VERIFY":
			continue
		}
		a.State = "ABORTED"
		a.Result = "Backend restarted; approval invalidated. Physical output state unconfirmed; no command retried."
		a.Events = append(a.Events, patchcontrol.Event{State: a.State, AtMS: now, Detail: a.Result})
		pending = append(pending, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range pending {
		b, err := json.Marshal(a)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE patch_actions SET payload=? WHERE id=?", string(b), a.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
