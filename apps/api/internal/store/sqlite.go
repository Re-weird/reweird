package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
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
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (store *SQLiteStore) Close() error { return store.db.Close() }
