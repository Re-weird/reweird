package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var _ domain.PassportRepository = (*SQLiteStore)(nil)

func (store *SQLiteStore) GetMeasurement(id int64) (*domain.MeasurementWindow, error) {
	if id <= 0 {
		return nil, nil
	}
	var window domain.MeasurementWindow
	var rawPayload, analysisPayload string
	err := store.db.QueryRow(`
		SELECT id, profile_id, source, device_id, sequence, captured_at_ms, ingested_at_ms, raw_payload, analysis_payload
		FROM measurement_windows WHERE id = ?
	`, id).Scan(&window.ID, &window.ProfileID, &window.Source, &window.DeviceID, &window.Sequence, &window.CapturedAtMS, &window.IngestedAtMS, &rawPayload, &analysisPayload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(rawPayload), &window.Raw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(analysisPayload), &window.Analysis); err != nil {
		return nil, err
	}
	return &window, nil
}

func (store *SQLiteStore) SaveKnownGood(record domain.KnownGoodBaseline) (domain.KnownGoodBaseline, error) {
	if record.ProfileID == "" || record.MeasurementID <= 0 || record.SavedAtMS <= 0 || record.DeviceID == "" {
		return domain.KnownGoodBaseline{}, fmt.Errorf("incomplete known-good record")
	}
	if record.Source != domain.BaselinePhysical && record.Source != domain.BaselineSimulated {
		return domain.KnownGoodBaseline{}, fmt.Errorf("invalid known-good source")
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return domain.KnownGoodBaseline{}, err
	}
	result, err := store.db.Exec(`
		INSERT INTO known_good_baselines
			(profile_id, measurement_id, source_kind, device_id, payload, saved_at_ms)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(measurement_id) DO NOTHING
	`, record.ProfileID, record.MeasurementID, record.Source, record.DeviceID, string(payload), record.SavedAtMS)
	if err != nil {
		return domain.KnownGoodBaseline{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return domain.KnownGoodBaseline{}, err
	}
	if count == 0 {
		return domain.KnownGoodBaseline{}, domain.ErrKnownGoodExists
	}
	record.ID, err = result.LastInsertId()
	return record, err
}

func (store *SQLiteStore) ListKnownGood(profileID string, limit int) ([]domain.KnownGoodBaseline, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := store.db.Query(`SELECT id, payload FROM known_good_baselines WHERE profile_id = ? ORDER BY saved_at_ms DESC, id DESC LIMIT ?`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.KnownGoodBaseline, 0)
	for rows.Next() {
		var id int64
		var payload string
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, err
		}
		var record domain.KnownGoodBaseline
		if err := json.Unmarshal([]byte(payload), &record); err != nil {
			return nil, err
		}
		record.ID = id
		items = append(items, record)
	}
	return items, rows.Err()
}

func (store *SQLiteStore) LatestKnownGood(profileID string, source domain.BaselineSource, deviceID string) (*domain.KnownGoodBaseline, error) {
	var id int64
	var payload string
	err := store.db.QueryRow(`
		SELECT id, payload FROM known_good_baselines
		WHERE profile_id = ? AND source_kind = ? AND device_id = ?
		ORDER BY saved_at_ms DESC, id DESC LIMIT 1
	`, profileID, source, deviceID).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record domain.KnownGoodBaseline
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, err
	}
	record.ID = id
	return &record, nil
}

func (store *SQLiteStore) LatestKnownGoodOfSource(profileID string, source domain.BaselineSource) (*domain.KnownGoodBaseline, error) {
	var id int64
	var payload string
	err := store.db.QueryRow(`
		SELECT id, payload FROM known_good_baselines
		WHERE profile_id = ? AND source_kind = ?
		ORDER BY saved_at_ms DESC, id DESC LIMIT 1
	`, profileID, source).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record domain.KnownGoodBaseline
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, err
	}
	record.ID = id
	return &record, nil
}

func (store *SQLiteStore) HasPhysicalBaseline(profileID string) (bool, error) {
	var count int
	err := store.db.QueryRow(`SELECT COUNT(*) FROM known_good_baselines WHERE profile_id = ? AND source_kind = ?`, profileID, domain.BaselinePhysical).Scan(&count)
	return count > 0, err
}
