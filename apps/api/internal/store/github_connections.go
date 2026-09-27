package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

var _ domain.GitHubConnectionRepository = (*SQLiteStore)(nil)

func (store *SQLiteStore) migrateGitHub() error {
	_, err := store.db.Exec(`
		CREATE TABLE IF NOT EXISTS github_connections (
			owner_id TEXT PRIMARY KEY,
			installation_id INTEGER NOT NULL,
			account_login TEXT NOT NULL,
			account_type TEXT NOT NULL,
			connected_at_ms INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_github_connections_installation
			ON github_connections(installation_id);
	`)
	return err
}

func (store *SQLiteStore) SaveGitHubConnection(connection domain.GitHubConnection) error {
	if connection.ConnectedAtMS == 0 {
		connection.ConnectedAtMS = time.Now().UnixMilli()
	}
	_, err := store.db.Exec(`
		INSERT INTO github_connections (owner_id, installation_id, account_login, account_type, connected_at_ms)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(owner_id) DO UPDATE SET
			installation_id = excluded.installation_id,
			account_login = excluded.account_login,
			account_type = excluded.account_type,
			connected_at_ms = excluded.connected_at_ms`,
		connection.OwnerID, connection.InstallationID, connection.AccountLogin, connection.AccountType, connection.ConnectedAtMS)
	return err
}

func (store *SQLiteStore) GetGitHubConnection(ownerID string) (*domain.GitHubConnection, error) {
	connection := domain.GitHubConnection{OwnerID: ownerID}
	err := store.db.QueryRow(
		"SELECT installation_id, account_login, account_type, connected_at_ms FROM github_connections WHERE owner_id = ?", ownerID,
	).Scan(&connection.InstallationID, &connection.AccountLogin, &connection.AccountType, &connection.ConnectedAtMS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

func (store *SQLiteStore) DeleteGitHubConnection(ownerID string) error {
	_, err := store.db.Exec("DELETE FROM github_connections WHERE owner_id = ?", ownerID)
	return err
}

func (store *SQLiteStore) DeleteGitHubConnectionsForInstallation(installationID int64) error {
	_, err := store.db.Exec("DELETE FROM github_connections WHERE installation_id = ?", installationID)
	return err
}
