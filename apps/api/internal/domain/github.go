package domain

// GitHubConnection records which GitHub App installation a ReWeird user
// connected in Settings. Only the installation id is kept; ReWeird never
// stores a GitHub user token. Installation tokens are minted per request
// from the App's private key and expire within the hour.
type GitHubConnection struct {
	// OwnerID is the verified ReWeird user id (empty for the single local
	// user when sign-in isn't configured). Never taken from the client.
	OwnerID        string `json:"-"`
	InstallationID int64  `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	AccountType    string `json:"account_type"`
	// GitHubUser is the GitHub login that approved the connection, which
	// can differ from AccountLogin for an organization installation.
	GitHubUser    string `json:"github_user"`
	ConnectedAtMS int64  `json:"connected_at_ms"`
}

type GitHubConnectionRepository interface {
	SaveGitHubConnection(connection GitHubConnection) error
	GetGitHubConnection(ownerID string) (*GitHubConnection, error)
	DeleteGitHubConnection(ownerID string) error
	// DeleteGitHubConnectionsForInstallation runs when GitHub reports the
	// App was uninstalled, so no owner keeps pointing at a dead install.
	DeleteGitHubConnectionsForInstallation(installationID int64) error
}

type RepositorySyncStatus string

const (
	RepositorySyncPending RepositorySyncStatus = "PENDING"
	RepositorySyncRunning RepositorySyncStatus = "SYNCING"
	RepositorySyncOK      RepositorySyncStatus = "SYNCED"
	RepositorySyncFailed  RepositorySyncStatus = "FAILED"
	// RepositorySyncBlocked means a new commit reached the default branch
	// after the Project Profile was confirmed. Confirmed profiles are
	// immutable, so the commit is recorded but not re-analyzed.
	RepositorySyncBlocked RepositorySyncStatus = "BLOCKED"
)

// LinkedRepository is the GitHub repo a project's code comes from. Pushes to
// DefaultBranch trigger a fresh code analysis.
type LinkedRepository struct {
	ID             int64                `json:"id"`
	FullName       string               `json:"full_name"`
	DefaultBranch  string               `json:"default_branch"`
	HTMLURL        string               `json:"html_url"`
	Private        bool                 `json:"private"`
	InstallationID int64                `json:"installation_id"`
	LastCommit     *RepositoryCommit    `json:"last_commit,omitempty"`
	LatestSeenSHA  string               `json:"latest_seen_sha,omitempty"`
	SyncStatus     RepositorySyncStatus `json:"sync_status"`
	SyncError      string               `json:"sync_error,omitempty"`
	SyncedAtMS     int64                `json:"synced_at_ms,omitempty"`
	AnalyzedFiles  []string             `json:"analyzed_files,omitempty"`
	SkippedFiles   int                  `json:"skipped_files,omitempty"`
}

// RepositoryCommit is the commit whose code the current analysis came from.
type RepositoryCommit struct {
	SHA           string `json:"sha"`
	Message       string `json:"message"`
	AuthorName    string `json:"author_name"`
	CommittedAtMS int64  `json:"committed_at_ms"`
	HTMLURL       string `json:"html_url"`
}
