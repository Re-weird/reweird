package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/re-weird/reweird/apps/api/internal/codeanalysis"
	"github.com/re-weird/reweird/apps/api/internal/diagnostics"
	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/githubapp"
	"github.com/re-weird/reweird/apps/api/internal/profiles"
	"github.com/re-weird/reweird/apps/api/internal/projectunderstanding"
	"github.com/re-weird/reweird/apps/api/internal/signalanalysis"
	"github.com/re-weird/reweird/apps/api/internal/simulator"
	"github.com/re-weird/reweird/apps/api/internal/store"
	"github.com/re-weird/reweird/apps/api/internal/vision"
	componentcatalog "github.com/re-weird/reweird/packages/component-catalog"
)

// fakeGitHub serves the handful of GitHub endpoints ReWeird calls. The only
// installation is 7 (account "octo"), which can see octo/rig.
type fakeGitHub struct {
	mu    sync.Mutex
	head  string
	files map[string]map[string]string // sha -> path -> contents
}

func (fake *fakeGitHub) push(sha string, files map[string]string) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.head = sha
	fake.files[sha] = files
}

func (fake *fakeGitHub) handler() http.Handler {
	mux := http.NewServeMux()
	write := func(writer http.ResponseWriter, value any) { _ = json.NewEncoder(writer).Encode(value) }
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(writer http.ResponseWriter, _ *http.Request) {
		write(writer, map[string]any{"token": "inst-token", "expires_at": time.Now().Add(time.Hour)})
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(writer http.ResponseWriter, request *http.Request) {
		_ = request.ParseForm()
		if request.Form.Get("code") == "good-code" {
			write(writer, map[string]string{"access_token": "user-token"})
			return
		}
		write(writer, map[string]string{"error": "bad_verification_code"})
	})
	mux.HandleFunc("GET /user/installations", func(writer http.ResponseWriter, _ *http.Request) {
		write(writer, map[string]any{"installations": []map[string]any{{"id": 7, "account": map[string]string{"login": "octo", "type": "User"}}}})
	})
	repo := map[string]any{"id": 501, "name": "rig", "full_name": "octo/rig", "default_branch": "main", "html_url": "https://github.com/octo/rig", "private": true}
	mux.HandleFunc("GET /installation/repositories", func(writer http.ResponseWriter, _ *http.Request) {
		write(writer, map[string]any{"repositories": []any{repo}})
	})
	mux.HandleFunc("GET /repos/octo/rig", func(writer http.ResponseWriter, _ *http.Request) { write(writer, repo) })
	mux.HandleFunc("GET /repos/octo/rig/commits/{ref}", func(writer http.ResponseWriter, request *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		sha := request.PathValue("ref")
		if sha == "main" {
			sha = fake.head
		}
		if _, ok := fake.files[sha]; !ok {
			http.NotFound(writer, request)
			return
		}
		write(writer, map[string]any{"sha": sha, "html_url": "https://github.com/octo/rig/commit/" + sha, "commit": map[string]any{"message": "Wire trigger pin\n\nbody", "author": map[string]any{"name": "Octo", "date": time.Now()}}})
	})
	mux.HandleFunc("GET /repos/octo/rig/git/trees/{sha}", func(writer http.ResponseWriter, request *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		tree := []map[string]any{}
		for path, body := range fake.files[request.PathValue("sha")] {
			tree = append(tree, map[string]any{"path": path, "type": "blob", "size": len(body)})
		}
		write(writer, map[string]any{"tree": tree})
	})
	mux.HandleFunc("GET /repos/octo/rig/contents/{path...}", func(writer http.ResponseWriter, request *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		body, ok := fake.files[request.URL.Query().Get("ref")][request.PathValue("path")]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(body))
	})
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	})
	return mux
}

const sketch = "const int TRIG_PIN = 5;\nconst int ECHO_PIN = 18;\nvoid setup() { pinMode(TRIG_PIN, OUTPUT); pinMode(ECHO_PIN, INPUT); }\n"

func testGitHubApp(t *testing.T) (*fiber.App, *githubapp.Client, *fakeGitHub, *store.SQLiteStore) {
	t.Helper()
	fake := &fakeGitHub{files: map[string]map[string]string{}}
	fake.push("sha-1", map[string]string{"rig.ino": sketch, "include/arduino_secrets.h": "#define PASS \"hunter2\"\n"})
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	client := githubapp.New(githubapp.Config{AppID: 42, Slug: "reweird", ClientID: "cid", ClientSecret: "shh", PrivateKey: key, WebhookSecret: "hook", APIURL: server.URL, WebURL: server.URL}, server.Client())

	repository, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	demo := profiles.UltrasonicDemo()
	if err := repository.SaveProfile(demo); err != nil {
		t.Fatal(err)
	}
	catalog, err := componentcatalog.Load()
	if err != nil {
		t.Fatal(err)
	}
	services := ProjectServices{Understanding: projectunderstanding.New(codeanalysis.New(), vision.SkippedAnalyzer{}, catalog), UploadRoot: t.TempDir(), GitHub: client}
	owner := func(ctx *fiber.Ctx) error {
		if value := ctx.Get("X-Test-Owner"); value != "" {
			ctx.Locals(ownerIDLocalsKey, value)
		}
		return ctx.Next()
	}
	return newApp(diagnostics.NewEngine(signalanalysis.New()), repository, simulator.NewUltrasonicSource(), demo.ID, services, owner), client, fake, repository
}

func decodeGitHubBody[T any](t *testing.T, response *http.Response) T {
	t.Helper()
	defer response.Body.Close()
	var value T
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return value
}

func connectOcto(t *testing.T, app *fiber.App, client *githubapp.Client, owner string) {
	t.Helper()
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/github/connect", owner, map[string]any{"installation_id": 7, "code": "good-code", "state": client.SignState(owner)})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("connect status = %d", response.StatusCode)
	}
}

func TestGitHubStatusWhenNotConfigured(t *testing.T) {
	app := testOwnedApp(t)
	status := decodeGitHubBody[map[string]any](t, doJSONAs(t, app, http.MethodGet, "/api/v1/github/status", "owner-a", nil))
	if status["configured"] != false || status["connected"] != false {
		t.Fatalf("status = %#v", status)
	}
	if response := doJSONAs(t, app, http.MethodGet, "/api/v1/github/repos", "owner-a", nil); response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("repos without GitHub = %d, want 503", response.StatusCode)
	}
}

func TestGitHubConnectRequiresOwnStateAndValidCode(t *testing.T) {
	app, client, _, _ := testGitHubApp(t)
	for name, body := range map[string]map[string]any{
		"another owner's state": {"installation_id": 7, "code": "good-code", "state": client.SignState("owner-b")},
		"rejected code":         {"installation_id": 7, "code": "bad-code", "state": client.SignState("owner-a")},
		"not their install":     {"installation_id": 8, "code": "good-code", "state": client.SignState("owner-a")},
	} {
		if response := doJSONAs(t, app, http.MethodPost, "/api/v1/github/connect", "owner-a", body); response.StatusCode < 400 {
			t.Fatalf("%s: status = %d, want an error", name, response.StatusCode)
		}
	}
	connectOcto(t, app, client, "owner-a")
	status := decodeGitHubBody[map[string]any](t, doJSONAs(t, app, http.MethodGet, "/api/v1/github/status", "owner-a", nil))
	if status["connected"] != true || status["account_login"] != "octo" {
		t.Fatalf("status after connect = %#v", status)
	}
	other := decodeGitHubBody[map[string]any](t, doJSONAs(t, app, http.MethodGet, "/api/v1/github/status", "owner-b", nil))
	if other["connected"] != false {
		t.Fatal("a connection must belong only to the user who made it")
	}
	if response := doJSONAs(t, app, http.MethodPost, "/api/v1/github/disconnect", "owner-a", nil); response.StatusCode != http.StatusOK {
		t.Fatalf("disconnect status = %d", response.StatusCode)
	}
	if response := doJSONAs(t, app, http.MethodGet, "/api/v1/github/repos", "owner-a", nil); response.StatusCode != http.StatusConflict {
		t.Fatalf("repos after disconnect = %d, want 409", response.StatusCode)
	}
}

func TestProjectFromRepositoryAnalyzesDefaultBranchAndFollowsPushes(t *testing.T) {
	app, client, fake, repository := testGitHubApp(t)
	connectOcto(t, app, client, "owner-a")

	repos := decodeGitHubBody[struct {
		Items []map[string]any `json:"items"`
	}](t, doJSONAs(t, app, http.MethodGet, "/api/v1/github/repos", "owner-a", nil))
	if len(repos.Items) != 1 || repos.Items[0]["full_name"] != "octo/rig" {
		t.Fatalf("repos = %#v", repos.Items)
	}
	if response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-b", map[string]any{"name": "Rig", "controller": "ESP32", "logic_voltage": 3.3, "repository": "octo/rig"}); response.StatusCode != http.StatusConflict {
		t.Fatalf("a user without a GitHub connection linked a repo: status %d", response.StatusCode)
	}

	created := decodeGitHubBody[domain.Project](t, doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Rig", "controller": "ESP32", "logic_voltage": 3.3, "repository": "octo/rig"}))
	if created.Repository == nil || created.Repository.FullName != "octo/rig" || created.Repository.InstallationID != 7 || created.Repository.SyncStatus != domain.RepositorySyncPending {
		t.Fatalf("created repository = %#v", created.Repository)
	}

	response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+created.ID+"/sync", "owner-a", nil)
	synced := decodeGitHubBody[struct {
		Project domain.Project         `json:"project"`
		Profile *domain.ProjectProfile `json:"profile"`
	}](t, response)
	if response.StatusCode != http.StatusOK || synced.Profile == nil {
		t.Fatalf("sync status = %d, profile = %v", response.StatusCode, synced.Profile)
	}
	linked := synced.Project.Repository
	if linked.SyncStatus != domain.RepositorySyncOK || linked.LastCommit == nil || linked.LastCommit.SHA != "sha-1" || linked.LastCommit.Message != "Wire trigger pin" {
		t.Fatalf("linked after sync = %#v", linked)
	}
	if strings.Join(linked.AnalyzedFiles, ",") != "rig.ino" || linked.SkippedFiles != 1 {
		t.Fatalf("analyzed %v, skipped %d; the secrets header must be skipped", linked.AnalyzedFiles, linked.SkippedFiles)
	}
	if synced.Project.Code == nil || strings.Contains(synced.Project.Code.Text, "hunter2") || synced.Project.AnalysisStatus != domain.AnalysisDraftReady {
		t.Fatalf("project code/status = %#v, %s", synced.Project.Code, synced.Project.AnalysisStatus)
	}
	if response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+created.ID+"/sync", "owner-b", nil); response.StatusCode != http.StatusNotFound {
		t.Fatalf("another owner could sync the project: %d", response.StatusCode)
	}

	fake.push("sha-2", map[string]string{"rig.ino": sketch + "// tuned\n"})
	payload := []byte(`{"ref":"refs/heads/main","after":"sha-2","repository":{"id":501,"default_branch":"main"},"installation":{"id":7}}`)
	if response := sendWebhook(t, app, "push", payload, "wrong-secret"); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsigned webhook status = %d, want 401", response.StatusCode)
	}
	if response := sendWebhook(t, app, "push", payload, "hook"); response.StatusCode != http.StatusAccepted {
		t.Fatalf("webhook status = %d, want 202", response.StatusCode)
	}
	waitFor(t, func() bool {
		project, _ := repository.GetProject(created.ID)
		return project.Repository.LastCommit != nil && project.Repository.LastCommit.SHA == "sha-2" && project.Repository.SyncStatus == domain.RepositorySyncOK
	})

	fake.push("sha-3", map[string]string{"rig.ino": sketch})
	other := []byte(`{"ref":"refs/heads/feature","after":"sha-3","repository":{"id":501,"default_branch":"main"},"installation":{"id":7}}`)
	sendWebhook(t, app, "push", other, "hook")
	time.Sleep(100 * time.Millisecond)
	if project, _ := repository.GetProject(created.ID); project.Repository.LastCommit.SHA != "sha-2" {
		t.Fatal("a push to a non-default branch must not be analyzed")
	}
}

func TestSyncAfterConfirmationIsBlocked(t *testing.T) {
	app, client, fake, repository := testGitHubApp(t)
	connectOcto(t, app, client, "owner-a")
	created := decodeGitHubBody[domain.Project](t, doJSONAs(t, app, http.MethodPost, "/api/v1/projects", "owner-a", map[string]any{"name": "Rig", "controller": "ESP32", "logic_voltage": 3.3, "repository": "octo/rig"}))
	doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+created.ID+"/sync", "owner-a", nil).Body.Close()
	profile, err := repository.GetProfile(created.ID)
	if err != nil || profile == nil {
		t.Fatalf("profile = %v, %v", profile, err)
	}
	profile.Confirmed = true
	if err := repository.SaveProfile(*profile); err != nil {
		t.Fatal(err)
	}

	fake.push("sha-2", map[string]string{"rig.ino": sketch + "// changed\n"})
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/projects/"+created.ID+"/sync", "owner-a", nil)
	body := decodeGitHubBody[struct {
		Project domain.Project `json:"project"`
	}](t, response)
	linked := body.Project.Repository
	if response.StatusCode != http.StatusUnprocessableEntity || linked.SyncStatus != domain.RepositorySyncBlocked || linked.LatestSeenSHA != "sha-2" || linked.LastCommit.SHA != "sha-1" {
		t.Fatalf("status %d, linked = %#v; a confirmed profile's code must not change", response.StatusCode, linked)
	}
}

func sendWebhook(t *testing.T, app *fiber.App, event string, body []byte, secret string) *http.Response {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	request := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Event", event)
	request.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	response, err := app.Test(request, 10_000)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met within 5s")
}

func TestGitHubConnectWithoutInstallationIDUsesExistingInstall(t *testing.T) {
	app, client, _, _ := testGitHubApp(t)
	status := decodeGitHubBody[map[string]any](t, doJSONAs(t, app, http.MethodGet, "/api/v1/github/status", "owner-a", nil))
	if url, _ := status["authorize_url"].(string); !strings.Contains(url, "/login/oauth/authorize?client_id=cid") {
		t.Fatalf("authorize_url = %v", status["authorize_url"])
	}
	response := doJSONAs(t, app, http.MethodPost, "/api/v1/github/connect", "owner-a", map[string]any{"code": "good-code", "state": client.SignState("owner-a")})
	connected := decodeGitHubBody[map[string]any](t, response)
	if response.StatusCode != http.StatusOK || connected["connected"] != true || connected["account_login"] != "octo" {
		t.Fatalf("authorize-only connect = %d %#v", response.StatusCode, connected)
	}
	if response := doJSONAs(t, app, http.MethodPost, "/api/v1/github/connect", "owner-a", map[string]any{"code": "good-code", "state": client.SignState("owner-b")}); response.StatusCode != http.StatusForbidden {
		t.Fatalf("another owner's state must be refused, got %d", response.StatusCode)
	}
}
