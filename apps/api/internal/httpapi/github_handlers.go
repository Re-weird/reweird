package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/githubapp"
)

func (controller *Controller) githubConnections() (domain.GitHubConnectionRepository, bool) {
	connections, ok := controller.repository.(domain.GitHubConnectionRepository)
	return connections, ok && controller.github != nil
}

func githubNotConfigured(ctx *fiber.Ctx) error {
	return apiError(ctx, fiber.StatusServiceUnavailable, "GITHUB_NOT_CONFIGURED", "GitHub isn't set up on this ReWeird server. Add the GitHub App settings to the API environment.")
}

func (controller *Controller) githubStatus(ctx *fiber.Ctx) error {
	connections, ok := controller.githubConnections()
	if !ok {
		return ctx.JSON(fiber.Map{"configured": false, "connected": false})
	}
	owner := ownerID(ctx)
	connection, err := connections.GetGitHubConnection(owner)
	if err != nil {
		return internalError(ctx, err)
	}
	state := controller.github.SignState(owner)
	response := fiber.Map{"configured": true, "connected": connection != nil, "install_url": controller.github.InstallURL(state), "authorize_url": controller.github.AuthorizeURL(state)}
	if connection != nil {
		response["account_login"] = connection.AccountLogin
		response["github_user"] = connection.GitHubUser
		response["account_type"] = connection.AccountType
		response["connected_at_ms"] = connection.ConnectedAtMS
	}
	return ctx.JSON(response)
}

// githubConnect finishes the install round trip. GitHub redirects to the
// web app's setup page with installation_id, an OAuth code, and our state;
// the page forwards them here. The installation is only saved after GitHub
// confirms, via the code, that this user can access it.
func (controller *Controller) githubConnect(ctx *fiber.Ctx) error {
	connections, ok := controller.githubConnections()
	if !ok {
		return githubNotConfigured(ctx)
	}
	var input struct {
		InstallationID int64  `json:"installation_id"`
		Code           string `json:"code"`
		State          string `json:"state"`
	}
	if err := ctx.BodyParser(&input); err != nil || input.InstallationID < 0 {
		return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "code and state are required; installation_id is optional.")
	}
	owner := ownerID(ctx)
	if !controller.github.VerifyState(input.State, owner) {
		return apiError(ctx, fiber.StatusForbidden, "INVALID_STATE", "This GitHub connection link expired or was started by a different session. Connect again from Settings.")
	}
	if strings.TrimSpace(input.Code) == "" {
		return apiError(ctx, fiber.StatusBadRequest, "GITHUB_CODE_REQUIRED", "GitHub didn't return an authorization code. Enable \"Request user authorization (OAuth) during installation\" on the GitHub App.")
	}
	var installation *githubapp.Installation
	var githubUser string
	var err error
	if input.InstallationID == 0 {
		// Authorization only (the App may already be installed): use the
		// user's existing installation, preferring their personal account.
		var grant *githubapp.UserGrant
		grant, err = controller.github.UserInstallations(ctx.Context(), input.Code)
		var installations []githubapp.Installation
		if grant != nil {
			installations, githubUser = grant.Installations, grant.Login
		}
		if err == nil && len(installations) == 0 {
			return ctx.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "INSTALL_REQUIRED", "detail": "Install the ReWeird GitHub App on your account to choose repositories.", "install_url": controller.github.InstallURL(controller.github.SignState(owner))})
		}
		// Prefer the approving user's own account, then any personal
		// account, then an organization.
		for index := range installations {
			candidate := &installations[index]
			if installation == nil || rankInstallation(candidate, githubUser) < rankInstallation(installation, githubUser) {
				installation = candidate
			}
		}
	} else {
		installation, githubUser, err = controller.github.VerifyUserInstallation(ctx.Context(), input.Code, input.InstallationID)
	}
	if errors.Is(err, githubapp.ErrNotFound) {
		return apiError(ctx, fiber.StatusForbidden, "INSTALLATION_NOT_YOURS", "That GitHub installation isn't accessible to the GitHub account that authorized it.")
	}
	if err != nil {
		log.Printf("github connect: %v", err)
		return apiError(ctx, fiber.StatusBadGateway, "GITHUB_UNAVAILABLE", "GitHub couldn't confirm the installation. Try connecting again.")
	}
	connection := domain.GitHubConnection{OwnerID: owner, InstallationID: installation.ID, AccountLogin: installation.Account.Login, AccountType: installation.Account.Type, GitHubUser: githubUser, ConnectedAtMS: time.Now().UnixMilli()}
	log.Printf("github connect: ReWeird user linked to installation %d (%s) approved by GitHub user %q", installation.ID, installation.Account.Login, githubUser)
	if err := connections.SaveGitHubConnection(connection); err != nil {
		return internalError(ctx, err)
	}
	return controller.githubStatus(ctx)
}

func rankInstallation(installation *githubapp.Installation, githubUser string) int {
	switch {
	case githubUser != "" && strings.EqualFold(installation.Account.Login, githubUser):
		return 0
	case installation.Account.Type == "User":
		return 1
	default:
		return 2
	}
}

// githubDisconnect forgets the installation for this user. The App stays
// installed on GitHub until the user removes it there.
func (controller *Controller) githubDisconnect(ctx *fiber.Ctx) error {
	connections, ok := controller.githubConnections()
	if !ok {
		return githubNotConfigured(ctx)
	}
	if err := connections.DeleteGitHubConnection(ownerID(ctx)); err != nil {
		return internalError(ctx, err)
	}
	return controller.githubStatus(ctx)
}

// currentGitHubConnection returns the caller's connection. When it returns
// nil with a nil error, an error response has already been written.
func (controller *Controller) currentGitHubConnection(ctx *fiber.Ctx) (*domain.GitHubConnection, error) {
	connections, ok := controller.githubConnections()
	if !ok {
		return nil, githubNotConfigured(ctx)
	}
	connection, err := connections.GetGitHubConnection(ownerID(ctx))
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, apiError(ctx, fiber.StatusConflict, "GITHUB_NOT_CONNECTED", "Connect GitHub in Settings first.")
	}
	return connection, nil
}

func (controller *Controller) githubRepositories(ctx *fiber.Ctx) error {
	connection, err := controller.currentGitHubConnection(ctx)
	if err != nil {
		return internalError(ctx, err)
	}
	if connection == nil {
		return nil
	}
	repositories, err := controller.github.ListRepositories(ctx.Context(), connection.InstallationID)
	if errors.Is(err, githubapp.ErrNotFound) {
		return apiError(ctx, fiber.StatusConflict, "GITHUB_NOT_CONNECTED", "The GitHub App was removed from your account. Connect GitHub again in Settings.")
	}
	if err != nil {
		log.Printf("github repositories: %v", err)
		return apiError(ctx, fiber.StatusBadGateway, "GITHUB_UNAVAILABLE", "Couldn't load your repositories from GitHub.")
	}
	sort.SliceStable(repositories, func(i, j int) bool { return repositories[i].PushedAt.After(repositories[j].PushedAt) })
	items := make([]fiber.Map, 0, len(repositories))
	for _, repository := range repositories {
		items = append(items, fiber.Map{
			"id": repository.ID, "name": repository.Name, "full_name": repository.FullName, "description": repository.Description,
			"private": repository.Private, "default_branch": repository.DefaultBranch, "html_url": repository.HTMLURL,
			"language": repository.Language, "pushed_at_ms": repository.PushedAt.UnixMilli(), "archived": repository.Archived,
		})
	}
	return ctx.JSON(fiber.Map{"account_login": connection.AccountLogin, "items": items})
}

// linkRepository resolves a create request's repo through the caller's own
// installation, so a project can only link a repo that user granted. When
// it returns nil with a nil error, an error response has been written.
func (controller *Controller) linkRepository(ctx *fiber.Ctx, fullName string) (*domain.LinkedRepository, error) {
	connection, err := controller.currentGitHubConnection(ctx)
	if connection == nil {
		return nil, err
	}
	repository, err := controller.github.GetRepository(ctx.Context(), connection.InstallationID, fullName)
	if errors.Is(err, githubapp.ErrNotFound) {
		return nil, apiError(ctx, fiber.StatusUnprocessableEntity, "REPOSITORY_NOT_ACCESSIBLE", "ReWeird can't see that repository. Grant the GitHub App access to it, then try again.")
	}
	if err != nil {
		log.Printf("github link repository: %v", err)
		return nil, apiError(ctx, fiber.StatusBadGateway, "GITHUB_UNAVAILABLE", "Couldn't reach GitHub to link the repository.")
	}
	return &domain.LinkedRepository{
		ID: repository.ID, FullName: repository.FullName, DefaultBranch: repository.DefaultBranch, HTMLURL: repository.HTMLURL,
		Private: repository.Private, InstallationID: connection.InstallationID, SyncStatus: domain.RepositorySyncPending,
	}, nil
}

// syncProjectRepository re-reads the linked repo's default branch now. It
// is the manual counterpart of the push webhook and the poller.
func (controller *Controller) syncProjectRepository(ctx *fiber.Ctx) error {
	project, err := controller.findProject(ctx, ctx.Params("id"))
	if err != nil {
		return internalError(ctx, err)
	}
	if project == nil {
		return apiError(ctx, fiber.StatusNotFound, "PROJECT_NOT_FOUND", "The requested project does not exist.")
	}
	if project.Repository == nil {
		return apiError(ctx, fiber.StatusConflict, "NO_REPOSITORY", "This project isn't linked to a GitHub repository.")
	}
	if controller.github == nil {
		return githubNotConfigured(ctx)
	}
	syncContext, cancel := context.WithTimeout(ctx.Context(), 90*time.Second)
	defer cancel()
	result, err := controller.syncRepository(syncContext, project.ID, "", true)
	if err != nil {
		return internalError(ctx, err)
	}
	status := fiber.StatusOK
	if result.project.Repository != nil && result.project.Repository.SyncStatus != domain.RepositorySyncOK {
		status = fiber.StatusUnprocessableEntity
	}
	body := fiber.Map{"project": result.project}
	if result.profile != nil {
		body["analysis"] = result.analysis
		body["profile"] = result.profile
	}
	return ctx.Status(status).JSON(body)
}

type syncResult struct {
	project  *domain.Project
	analysis *domain.ProjectAnalysis
	profile  *domain.ProjectProfile
}

// syncRepository analyzes the linked repo at ref (the default branch when
// empty). force re-runs even if that commit was already analyzed. Syncs
// are serialized by syncMu; profileMu is only held around reads and writes
// so slow GitHub calls don't block other project requests.
func (controller *Controller) syncRepository(ctx context.Context, projectID, ref string, force bool) (*syncResult, error) {
	controller.syncMu.Lock()
	defer controller.syncMu.Unlock()

	project, err := controller.repository.GetProject(projectID)
	if err != nil || project == nil || project.Repository == nil {
		if err == nil {
			err = errors.New("project or linked repository disappeared")
		}
		return nil, err
	}
	linked := *project.Repository
	if ref == "" {
		ref = linked.DefaultBranch
	}
	commit, err := controller.github.Commit(ctx, linked.InstallationID, linked.FullName, ref)
	if err != nil {
		message := "Couldn't read the default branch from GitHub."
		if errors.Is(err, githubapp.ErrNotFound) {
			message = "GitHub no longer shows this repository or branch to ReWeird. Check the App's repository access."
		}
		log.Printf("github sync %s: %v", projectID, err)
		return controller.recordSync(projectID, func(repository *domain.LinkedRepository) {
			repository.SyncStatus, repository.SyncError = domain.RepositorySyncFailed, message
		})
	}
	if !force && linked.LastCommit != nil && linked.LastCommit.SHA == commit.SHA && linked.SyncStatus == domain.RepositorySyncOK {
		return &syncResult{project: project}, nil
	}
	confirmed, err := controller.projectProfileConfirmed(projectID)
	if err != nil {
		return nil, err
	}
	if confirmed {
		return controller.recordSync(projectID, func(repository *domain.LinkedRepository) {
			repository.LatestSeenSHA = commit.SHA
			repository.SyncStatus = domain.RepositorySyncBlocked
			repository.SyncError = "A new commit reached " + linked.DefaultBranch + " after the profile was confirmed. Confirmed profiles don't change; start a new revision to analyze it."
		})
	}
	if _, err := controller.recordSync(projectID, func(repository *domain.LinkedRepository) {
		repository.LatestSeenSHA, repository.SyncStatus, repository.SyncError = commit.SHA, domain.RepositorySyncRunning, ""
	}); err != nil {
		return nil, err
	}

	bundle, collectErr := controller.github.CollectSource(ctx, linked.InstallationID, linked.FullName, commit.SHA)
	if collectErr != nil {
		message := collectErr.Error()
		if !errors.Is(collectErr, githubapp.ErrNoSource) {
			log.Printf("github collect %s: %v", projectID, collectErr)
			message = "Couldn't read the source files from GitHub."
		}
		return controller.recordSync(projectID, func(repository *domain.LinkedRepository) {
			repository.SyncStatus, repository.SyncError = domain.RepositorySyncFailed, message
		})
	}

	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err = controller.repository.GetProject(projectID)
	if err != nil || project == nil || project.Repository == nil {
		return nil, errors.New("project changed during sync")
	}
	if confirmed, err := controller.projectProfileConfirmed(projectID); err != nil || confirmed {
		if err != nil {
			return nil, err
		}
		project.Repository.SyncStatus = domain.RepositorySyncBlocked
		project.Repository.SyncError = "The profile was confirmed while this commit was being read."
		return &syncResult{project: project}, controller.repository.SaveProject(*project)
	}
	project.Code = bundle.Code
	resetProjectAnalysis(project)
	project.Repository.LastCommit = &domain.RepositoryCommit{
		SHA: commit.SHA, Message: firstLine(commit.Commit.Message), AuthorName: commit.Commit.Author.Name,
		CommittedAtMS: commit.Commit.Author.Date.UnixMilli(), HTMLURL: commit.HTMLURL,
	}
	project.Repository.AnalyzedFiles, project.Repository.SkippedFiles = bundle.Files, bundle.Skipped
	project.Repository.SyncStatus, project.Repository.SyncError = domain.RepositorySyncOK, ""
	project.Repository.SyncedAtMS = time.Now().UnixMilli()
	if controller.understanding == nil {
		return &syncResult{project: project}, controller.repository.SaveProject(*project)
	}
	analysis, profile, failure, err := controller.analyzeLocked(ctx, project)
	if err != nil {
		return nil, err
	}
	if failure != nil {
		project.Repository.SyncStatus, project.Repository.SyncError = domain.RepositorySyncFailed, failure.message
		return &syncResult{project: project}, controller.repository.SaveProject(*project)
	}
	stored, err := controller.repository.GetProject(projectID)
	if err != nil {
		return nil, err
	}
	return &syncResult{project: stored, analysis: &analysis, profile: &profile}, nil
}

func (controller *Controller) recordSync(projectID string, change func(*domain.LinkedRepository)) (*syncResult, error) {
	controller.profileMu.Lock()
	defer controller.profileMu.Unlock()
	project, err := controller.repository.GetProject(projectID)
	if err != nil || project == nil || project.Repository == nil {
		return nil, errors.Join(err, errors.New("project or linked repository disappeared"))
	}
	change(project.Repository)
	return &syncResult{project: project}, controller.repository.SaveProject(*project)
}

func firstLine(message string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}

// githubWebhook receives GitHub App events. It is mounted outside /api/v1
// because GitHub can't send the API bearer token; the HMAC signature is the
// only credential. Analysis runs after the 202 so GitHub isn't kept waiting.
func (controller *Controller) githubWebhook(ctx *fiber.Ctx) error {
	if controller.github == nil {
		return githubNotConfigured(ctx)
	}
	body := ctx.Body()
	if !controller.github.VerifyWebhookSignature(body, ctx.Get("X-Hub-Signature-256")) {
		return apiError(ctx, fiber.StatusUnauthorized, "INVALID_SIGNATURE", "Webhook signature did not verify.")
	}
	switch ctx.Get("X-GitHub-Event") {
	case "ping":
		return ctx.JSON(fiber.Map{"ok": true})
	case "installation":
		var event struct {
			Action       string `json:"action"`
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
		}
		if json.Unmarshal(body, &event) == nil && event.Action == "deleted" {
			controller.github.Forget(event.Installation.ID)
			if connections, ok := controller.githubConnections(); ok {
				if err := connections.DeleteGitHubConnectionsForInstallation(event.Installation.ID); err != nil {
					return internalError(ctx, err)
				}
			}
		}
		return ctx.SendStatus(fiber.StatusAccepted)
	case "push":
		var event struct {
			Ref        string `json:"ref"`
			After      string `json:"after"`
			Deleted    bool   `json:"deleted"`
			Repository struct {
				ID            int64  `json:"id"`
				DefaultBranch string `json:"default_branch"`
			} `json:"repository"`
			Installation struct {
				ID int64 `json:"id"`
			} `json:"installation"`
		}
		if err := json.Unmarshal(body, &event); err != nil {
			return apiError(ctx, fiber.StatusBadRequest, "INVALID_JSON", "Push payload could not be parsed.")
		}
		if event.Deleted || event.Ref != "refs/heads/"+event.Repository.DefaultBranch {
			return ctx.SendStatus(fiber.StatusAccepted)
		}
		items, err := controller.repository.ListProjects()
		if err != nil {
			return internalError(ctx, err)
		}
		for _, project := range items {
			if project.Repository != nil && project.Repository.ID == event.Repository.ID && project.Repository.InstallationID == event.Installation.ID {
				go controller.backgroundSync(project.ID, event.After)
			}
		}
		return ctx.SendStatus(fiber.StatusAccepted)
	default:
		return ctx.SendStatus(fiber.StatusAccepted)
	}
}

func (controller *Controller) backgroundSync(projectID, ref string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := controller.syncRepository(ctx, projectID, ref, false); err != nil {
		log.Printf("github sync %s: %v", projectID, err)
	}
}

// PollRepositories is the fallback for servers GitHub can't reach (such as
// localhost): every interval it checks each linked repo's default branch
// and analyzes commits it hasn't seen yet.
func (controller *Controller) PollRepositories(ctx context.Context, interval time.Duration) {
	if controller.github == nil || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		items, err := controller.repository.ListProjects()
		if err != nil {
			log.Printf("github poll: %v", err)
			continue
		}
		for _, project := range items {
			linked := project.Repository
			if linked == nil || linked.SyncStatus == domain.RepositorySyncRunning {
				continue
			}
			checkContext, cancel := context.WithTimeout(ctx, 20*time.Second)
			commit, err := controller.github.Commit(checkContext, linked.InstallationID, linked.FullName, linked.DefaultBranch)
			cancel()
			// LatestSeenSHA covers commits that failed or were blocked, so a
			// bad commit isn't retried every tick; "Check now" forces it.
			if err != nil || commit.SHA == linked.LatestSeenSHA || linked.LastCommit != nil && commit.SHA == linked.LastCommit.SHA {
				continue
			}
			controller.backgroundSync(project.ID, commit.SHA)
		}
	}
}
