package store

import (
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestGitHubConnectionLifecycle(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "reweird-test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer repository.Close()

	if connection, err := repository.GetGitHubConnection("owner-a"); err != nil || connection != nil {
		t.Fatalf("GetGitHubConnection() before connect = %#v, %v", connection, err)
	}
	for _, connection := range []domain.GitHubConnection{
		{OwnerID: "owner-a", InstallationID: 11, AccountLogin: "old-login", AccountType: "User"},
		{OwnerID: "owner-a", InstallationID: 12, AccountLogin: "octo", AccountType: "User"},
		{OwnerID: "owner-b", InstallationID: 12, AccountLogin: "octo", AccountType: "User"},
		{OwnerID: "owner-c", InstallationID: 99, AccountLogin: "other", AccountType: "Organization"},
	} {
		if err := repository.SaveGitHubConnection(connection); err != nil {
			t.Fatalf("SaveGitHubConnection() error = %v", err)
		}
	}
	stored, err := repository.GetGitHubConnection("owner-a")
	if err != nil || stored == nil || stored.InstallationID != 12 || stored.AccountLogin != "octo" || stored.ConnectedAtMS == 0 {
		t.Fatalf("reconnect should replace the owner's row, got %#v, %v", stored, err)
	}

	if err := repository.DeleteGitHubConnectionsForInstallation(12); err != nil {
		t.Fatalf("DeleteGitHubConnectionsForInstallation() error = %v", err)
	}
	for owner, want := range map[string]bool{"owner-a": false, "owner-b": false, "owner-c": true} {
		connection, err := repository.GetGitHubConnection(owner)
		if err != nil || (connection != nil) != want {
			t.Fatalf("after uninstall %s connected = %v, want %v (err %v)", owner, connection != nil, want, err)
		}
	}
	if err := repository.DeleteGitHubConnection("owner-c"); err != nil {
		t.Fatalf("DeleteGitHubConnection() error = %v", err)
	}
	if connection, _ := repository.GetGitHubConnection("owner-c"); connection != nil {
		t.Fatalf("DeleteGitHubConnection() left %#v", connection)
	}
}
