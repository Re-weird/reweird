package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db            *sql.DB
	measurementMu sync.Mutex
}

func (store *SQLiteStore) Ping() error { return store.db.Ping() }

func MeasurementWindowLimit() int {
	value, err := strconv.Atoi(os.Getenv("MAX_MEASUREMENT_WINDOWS"))
	if err != nil || value < 100 || value > 1000000 {
		return 50000
	}
	return value
}

func (store *SQLiteStore) MeasurementWindowCount() (int, error) {
	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM measurement_windows").Scan(&count)
	return count, err
}

func Open(path string) (*SQLiteStore, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: database}
	if err := store.migrate(); err != nil {
		_ = database.Close()
		return nil, err
	}
	return store, nil
}

func (store *SQLiteStore) migrate() error {
	_, err := store.db.Exec(`
		CREATE TABLE IF NOT EXISTS diagnostic_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_key TEXT NOT NULL,
			stage TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_created_at
			ON diagnostic_sessions(created_at DESC);
		CREATE TABLE IF NOT EXISTS project_profiles (
			id TEXT PRIMARY KEY,
			payload TEXT NOT NULL,
			confirmed INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_profiles_updated_at
			ON project_profiles(updated_at DESC);
		CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			payload TEXT NOT NULL,
			analysis_status TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_projects_updated_at
			ON projects(updated_at DESC);
		CREATE TABLE IF NOT EXISTS measurement_windows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL,
			source TEXT NOT NULL,
			device_id TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			captured_at_ms INTEGER NOT NULL,
			ingested_at_ms INTEGER NOT NULL,
			raw_payload TEXT NOT NULL,
			analysis_payload TEXT NOT NULL,
			UNIQUE(source, device_id, sequence, captured_at_ms)
		);
		CREATE INDEX IF NOT EXISTS idx_measurements_profile_captured
			ON measurement_windows(profile_id, captured_at_ms DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_measurements_profile_ingested
			ON measurement_windows(profile_id, ingested_at_ms DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_measurements_device_captured
			ON measurement_windows(device_id, captured_at_ms DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_measurements_source_captured
			ON measurement_windows(source, captured_at_ms DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_measurements_captured
			ON measurement_windows(captured_at_ms DESC, id DESC);
		CREATE TABLE IF NOT EXISTS known_good_baselines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id TEXT NOT NULL,
			measurement_id INTEGER NOT NULL UNIQUE,
			source_kind TEXT NOT NULL,
			device_id TEXT NOT NULL,
			payload TEXT NOT NULL,
			saved_at_ms INTEGER NOT NULL,
			FOREIGN KEY(measurement_id) REFERENCES measurement_windows(id)
		);
		CREATE INDEX IF NOT EXISTS idx_known_good_profile_source
			ON known_good_baselines(profile_id, source_kind, device_id, saved_at_ms DESC, id DESC);
		CREATE TABLE IF NOT EXISTS test_workflows (
			id TEXT PRIMARY KEY,
			profile_id TEXT NOT NULL,
			status TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at_ms INTEGER NOT NULL,
			updated_at_ms INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_test_workflows_profile_updated
			ON test_workflows(profile_id, updated_at_ms DESC);
	`)
	return err
}

func (store *SQLiteStore) SaveSession(session domain.Session) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = store.db.Exec(
		"INSERT INTO diagnostic_sessions (session_key, stage, payload, created_at) VALUES (?, ?, ?, ?)",
		session.ID,
		session.Stage,
		string(payload),
		time.Now().UTC(),
	)
	return err
}

func (store *SQLiteStore) LatestSession() (*domain.Session, error) {
	var payload string
	err := store.db.QueryRow("SELECT payload FROM diagnostic_sessions ORDER BY created_at DESC, id DESC LIMIT 1").Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var session domain.Session
	if err := json.Unmarshal([]byte(payload), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (store *SQLiteStore) SaveMeasurement(window domain.MeasurementWindow) (domain.MeasurementWindow, error) {
	store.measurementMu.Lock()
	defer store.measurementMu.Unlock()
	count, err := store.MeasurementWindowCount()
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	if count >= MeasurementWindowLimit() {
		var existing int64
		err := store.db.QueryRow(`SELECT id FROM measurement_windows WHERE source = ? AND device_id = ? AND sequence = ? AND captured_at_ms = ?`, window.Source, window.DeviceID, window.Sequence, window.CapturedAtMS).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.MeasurementWindow{}, fmt.Errorf("measurement retention limit of %d windows reached; export/archive evidence before adding more", MeasurementWindowLimit())
		}
		if err != nil {
			return domain.MeasurementWindow{}, err
		}
	}
	if window.IngestedAtMS == 0 {
		window.IngestedAtMS = time.Now().UTC().UnixMilli()
	}
	rawPayload, err := json.Marshal(window.Raw)
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	analysisPayload, err := json.Marshal(window.Analysis)
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	result, err := store.db.Exec(`
		INSERT INTO measurement_windows
			(profile_id, source, device_id, sequence, captured_at_ms, ingested_at_ms, raw_payload, analysis_payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, device_id, sequence, captured_at_ms) DO NOTHING
	`, window.ProfileID, window.Source, window.DeviceID, window.Sequence, window.CapturedAtMS, window.IngestedAtMS, string(rawPayload), string(analysisPayload))
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	if inserted == 0 {
		var existingID int64
		if err := store.db.QueryRow(`
			SELECT id FROM measurement_windows
			WHERE source = ? AND device_id = ? AND sequence = ? AND captured_at_ms = ?
		`, window.Source, window.DeviceID, window.Sequence, window.CapturedAtMS).Scan(&existingID); err != nil {
			return domain.MeasurementWindow{}, err
		}
		existing, err := store.GetMeasurement(existingID)
		if err != nil {
			return domain.MeasurementWindow{}, err
		}
		if existing == nil {
			return domain.MeasurementWindow{}, errors.New("measurement identity disappeared during duplicate lookup")
		}
		originalRaw, err := json.Marshal(existing.Raw)
		if err != nil {
			return domain.MeasurementWindow{}, err
		}
		originalAnalysis, err := json.Marshal(existing.Analysis)
		if err != nil {
			return domain.MeasurementWindow{}, err
		}
		if string(originalRaw) != string(rawPayload) || string(originalAnalysis) != string(analysisPayload) || existing.ProfileID != window.ProfileID {
			return domain.MeasurementWindow{}, errors.New("telemetry identity collision: raw or derived evidence differs from an immutable stored measurement")
		}
		return *existing, nil
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.MeasurementWindow{}, err
	}
	window.ID = id
	return window, nil
}

func (store *SQLiteStore) ListMeasurements(profileID string, limit int) ([]domain.MeasurementWindow, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := store.db.Query(`
		SELECT id, profile_id, source, device_id, sequence, captured_at_ms, ingested_at_ms, raw_payload, analysis_payload
		FROM measurement_windows WHERE profile_id = ?
		ORDER BY ingested_at_ms DESC, id DESC LIMIT ?
	`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	windows := make([]domain.MeasurementWindow, 0)
	for rows.Next() {
		var window domain.MeasurementWindow
		var rawPayload, analysisPayload string
		if err := rows.Scan(&window.ID, &window.ProfileID, &window.Source, &window.DeviceID, &window.Sequence, &window.CapturedAtMS, &window.IngestedAtMS, &rawPayload, &analysisPayload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawPayload), &window.Raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(analysisPayload), &window.Analysis); err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	return windows, rows.Err()
}

// probeOverfetchLimit bounds how many extra rows QueryMeasurements will
// scan when a Probe filter is set (measurement_windows has no per-probe
// column to index on, so probe filtering is applied in Go after an
// indexed profile/device/source/time-range fetch) -- this can never
// become an unbounded scan even when very few matching windows exist.
const probeOverfetchLimit = 500

// QueryMeasurements supports the scoped telemetry reads ReWeird actually
// needs (a profile/session, a device or source, a time range, latest-first
// chronological order) directly against the existing measurement_windows
// table -- it does not introduce a second telemetry store. Every filter is
// optional; Limit is always clamped to the same [1, 200] bound as
// ListMeasurements.
func (store *SQLiteStore) QueryMeasurements(query domain.MeasurementQuery) ([]domain.MeasurementWindow, error) {
	limit := query.Limit
	if limit < 1 || limit > 200 {
		limit = 50
	}
	fetchLimit := limit
	if query.Probe != "" && fetchLimit < probeOverfetchLimit {
		// Probe isn't an indexed column, so overfetch (still bounded) and
		// filter in Go below, rather than scanning the whole table.
		fetchLimit = probeOverfetchLimit
	}

	conditions := make([]string, 0, 5)
	args := make([]any, 0, 6)
	if query.ProfileID != "" {
		conditions = append(conditions, "profile_id = ?")
		args = append(args, query.ProfileID)
	}
	if query.DeviceID != "" {
		conditions = append(conditions, "device_id = ?")
		args = append(args, query.DeviceID)
	}
	if query.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, query.Source)
	}
	if query.SinceMS != nil {
		conditions = append(conditions, "captured_at_ms >= ?")
		args = append(args, *query.SinceMS)
	}
	if query.UntilMS != nil {
		conditions = append(conditions, "captured_at_ms <= ?")
		args = append(args, *query.UntilMS)
	}
	where := "1 = 1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}
	args = append(args, fetchLimit)

	rows, err := store.db.Query(fmt.Sprintf(`
		SELECT id, profile_id, source, device_id, sequence, captured_at_ms, ingested_at_ms, raw_payload, analysis_payload
		FROM measurement_windows WHERE %s
		ORDER BY captured_at_ms DESC, id DESC LIMIT ?
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	windows := make([]domain.MeasurementWindow, 0)
	for rows.Next() {
		var window domain.MeasurementWindow
		var rawPayload, analysisPayload string
		if err := rows.Scan(&window.ID, &window.ProfileID, &window.Source, &window.DeviceID, &window.Sequence, &window.CapturedAtMS, &window.IngestedAtMS, &rawPayload, &analysisPayload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawPayload), &window.Raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(analysisPayload), &window.Analysis); err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if query.Probe == "" {
		return windows, nil
	}
	filtered := make([]domain.MeasurementWindow, 0, limit)
	for _, window := range windows {
		for _, sample := range window.Raw.Samples {
			if sample.Probe == query.Probe {
				filtered = append(filtered, window)
				break
			}
		}
		if len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}

func (store *SQLiteStore) SaveProfile(profile domain.ProjectProfile) error {
	now := time.Now().UTC()
	if profile.CreatedAtMS == 0 {
		profile.CreatedAtMS = now.UnixMilli()
	}
	profile.UpdatedAtMS = now.UnixMilli()
	payload, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	confirmed := 0
	if profile.Confirmed {
		confirmed = 1
	}
	_, err = store.db.Exec(`
		INSERT INTO project_profiles (id, payload, confirmed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload = excluded.payload,
			confirmed = excluded.confirmed,
			updated_at = excluded.updated_at
	`, profile.ID, string(payload), confirmed, time.UnixMilli(profile.CreatedAtMS).UTC(), now)
	return err
}

func (store *SQLiteStore) SaveProjectProfile(project domain.Project, profile domain.ProjectProfile) error {
	now := time.Now().UTC()
	if project.CreatedAtMS == 0 {
		project.CreatedAtMS = now.UnixMilli()
	}
	project.UpdatedAtMS = now.UnixMilli()
	if profile.CreatedAtMS == 0 {
		profile.CreatedAtMS = now.UnixMilli()
	}
	profile.UpdatedAtMS = now.UnixMilli()
	projectPayload, err := json.Marshal(project)
	if err != nil {
		return err
	}
	profilePayload, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	confirmed := 0
	if profile.Confirmed {
		confirmed = 1
	}
	tx, err := store.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec(`
		INSERT INTO project_profiles (id, payload, confirmed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload = excluded.payload,
			confirmed = excluded.confirmed,
			updated_at = excluded.updated_at
	`, profile.ID, string(profilePayload), confirmed, time.UnixMilli(profile.CreatedAtMS).UTC(), now); err != nil {
		return err
	}
	if _, err = tx.Exec(`
		INSERT INTO projects (id, payload, analysis_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload = excluded.payload,
			analysis_status = excluded.analysis_status,
			updated_at = excluded.updated_at
	`, project.ID, string(projectPayload), project.AnalysisStatus, time.UnixMilli(project.CreatedAtMS).UTC(), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *SQLiteStore) GetProfile(id string) (*domain.ProjectProfile, error) {
	var payload string
	err := store.db.QueryRow("SELECT payload FROM project_profiles WHERE id = ?", id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var profile domain.ProjectProfile
	if err := json.Unmarshal([]byte(payload), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (store *SQLiteStore) ListProfiles() ([]domain.ProjectProfile, error) {
	rows, err := store.db.Query("SELECT payload FROM project_profiles ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]domain.ProjectProfile, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var profile domain.ProjectProfile
		if err := json.Unmarshal([]byte(payload), &profile); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}

func (store *SQLiteStore) SaveProject(project domain.Project) error {
	now := time.Now().UTC()
	if project.CreatedAtMS == 0 {
		project.CreatedAtMS = now.UnixMilli()
	}
	project.UpdatedAtMS = now.UnixMilli()
	payload, err := json.Marshal(project)
	if err != nil {
		return err
	}
	_, err = store.db.Exec(`
		INSERT INTO projects (id, payload, analysis_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload = excluded.payload,
			analysis_status = excluded.analysis_status,
			updated_at = excluded.updated_at
	`, project.ID, string(payload), project.AnalysisStatus, time.UnixMilli(project.CreatedAtMS).UTC(), now)
	return err
}

// SetProjectVisibility updates only the visibility field, in one statement.
// Reading the whole payload and writing it back would let a concurrent
// SaveProject land in between and be overwritten by the stale copy.
func (store *SQLiteStore) SetProjectVisibility(id string, visibility domain.ProjectVisibility) error {
	result, err := store.db.Exec("UPDATE projects SET payload = json_set(payload, '$.visibility', ?) WHERE id = ?", string(visibility), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (store *SQLiteStore) GetProject(id string) (*domain.Project, error) {
	var payload string
	err := store.db.QueryRow("SELECT payload FROM projects WHERE id = ?", id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var project domain.Project
	if err := json.Unmarshal([]byte(payload), &project); err != nil {
		return nil, err
	}
	defaultVisibility(&project)
	return &project, nil
}

func (store *SQLiteStore) ListProjects() ([]domain.Project, error) {
	rows, err := store.db.Query("SELECT payload FROM projects ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]domain.Project, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var project domain.Project
		if err := json.Unmarshal([]byte(payload), &project); err != nil {
			return nil, err
		}
		defaultVisibility(&project)
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// Projects stored before visibility existed have no value; treat them as
// private, the safe default.
func defaultVisibility(project *domain.Project) {
	if project.Visibility == "" {
		project.Visibility = domain.VisibilityPrivate
	}
}

func (store *SQLiteStore) Close() error { return store.db.Close() }
