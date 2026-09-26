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

func (store *SQLiteStore) Close() error { return store.db.Close() }
