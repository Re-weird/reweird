package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"time"
)

func (store *SQLiteStore) SaveBridge(binding domain.BridgeBinding) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if binding.Raw != nil {
		var count, exists int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM bridge_capture_audit`).Scan(&count); err != nil {
			return err
		}
		if err = tx.QueryRow(`SELECT COUNT(*) FROM bridge_capture_audit WHERE project_id=? AND sent_at_ms=?`, binding.ProjectID, binding.SentAtMS).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 && count >= MeasurementWindowLimit() {
			return fmt.Errorf("bridge raw evidence retention limit reached; archive evidence before continuing")
		}
		raw, err := json.Marshal(binding.Raw)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO bridge_capture_audit(project_id,sent_at_ms,profile_hash,raw_payload) VALUES(?,?,?,?)`, binding.ProjectID, binding.SentAtMS, binding.ProfileHash, string(raw)); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO bridge_bindings(project_id,data) VALUES(?,?) ON CONFLICT(project_id) DO UPDATE SET data=excluded.data`, binding.ProjectID, string(data)); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *SQLiteStore) GetBridge(id string) (*domain.BridgeBinding, error) {
	var data string
	err := store.db.QueryRow(`SELECT data FROM bridge_bindings WHERE project_id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var b domain.BridgeBinding
	if err = json.Unmarshal([]byte(data), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (store *SQLiteStore) ActiveBridgeForDevice(deviceID string) (*domain.BridgeBinding, error) {
	var id string
	err := store.db.QueryRow(`SELECT project_id FROM bridge_bindings WHERE json_extract(data,'$.device_id')=? AND json_extract(data,'$.expires_at_ms')>? LIMIT 1`, deviceID, time.Now().UnixMilli()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return store.GetBridge(id)
}
