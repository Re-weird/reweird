package githubapp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type fakeGitHub struct {
	server      *httptest.Server
	tokenCalls  atomic.Int32
	files       map[string]string
	tree        []TreeEntry
	userInstall int64
}

func newFakeGitHub(t *testing.T, key *rsa.PrivateKey) *fakeGitHub {
	fake := &fakeGitHub{files: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/{id}/access_tokens", func(writer http.ResponseWriter, request *http.Request) {
		raw := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		claims := &jwt.RegisteredClaims{}
		if _, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return &key.PublicKey, nil }); err != nil || claims.Issuer != "42" {
			http.Error(writer, "bad app jwt", http.StatusUnauthorized)
			return
		}
		fake.tokenCalls.Add(1)
		json.NewEncoder(writer).Encode(map[string]any{"token": "inst-token", "expires_at": time.Now().Add(time.Hour)})
	})
	requireInstallToken := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(writer http.ResponseWriter, request *http.Request) {
			if request.Header.Get("Authorization") != "token inst-token" {
				http.Error(writer, "no token", http.StatusUnauthorized)
				return
			}
			handler(writer, request)
		}
	}
	mux.HandleFunc("GET /installation/repositories", requireInstallToken(func(writer http.ResponseWriter, request *http.Request) {
		json.NewEncoder(writer).Encode(map[string]any{"repositories": []map[string]any{{"id": 1, "full_name": "octo/rig", "name": "rig", "default_branch": "main"}}})
	}))
	mux.HandleFunc("GET /repos/octo/rig/git/trees/{sha}", requireInstallToken(func(writer http.ResponseWriter, request *http.Request) {
		json.NewEncoder(writer).Encode(map[string]any{"tree": fake.tree})
	}))
	mux.HandleFunc("GET /repos/octo/rig/contents/{path...}", requireInstallToken(func(writer http.ResponseWriter, request *http.Request) {
		body, ok := fake.files[request.PathValue("path")]
		if !ok {
			http.NotFound(writer, request)
			return
		}
		writer.Write([]byte(body))
	}))
	mux.HandleFunc("POST /login/oauth/access_token", func(writer http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		if request.Form.Get("code") != "good-code" || request.Form.Get("client_secret") != "shh" {
			json.NewEncoder(writer).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		json.NewEncoder(writer).Encode(map[string]string{"access_token": "user-token"})
	})
	mux.HandleFunc("GET /user/installations", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer user-token" {
			http.Error(writer, "no", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(writer).Encode(map[string]any{"installations": []map[string]any{{"id": fake.userInstall, "account": map[string]string{"login": "octo", "type": "User"}}}})
	})
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

func newTestClient(t *testing.T) (*Client, *fakeGitHub) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakeGitHub(t, key)
	client := New(Config{AppID: 42, Slug: "reweird", ClientID: "cid", ClientSecret: "shh", PrivateKey: key, WebhookSecret: "hook", APIURL: fake.server.URL, WebURL: fake.server.URL}, nil)
	return client, fake
}

func TestListRepositoriesMintsAndCachesInstallationToken(t *testing.T) {
	client, fake := newTestClient(t)
	for range 2 {
		repositories, err := client.ListRepositories(context.Background(), 7)
		if err != nil || len(repositories) != 1 || repositories[0].FullName != "octo/rig" {
			t.Fatalf("ListRepositories() = %#v, %v", repositories, err)
		}
	}
	if calls := fake.tokenCalls.Load(); calls != 1 {
		t.Fatalf("installation token minted %d times, want 1 (cached)", calls)
	}
}

func TestCollectSourceSkipsSecretsDependenciesAndOtherLanguage(t *testing.T) {
	client, fake := newTestClient(t)
	fake.tree = []TreeEntry{
		{Path: "src/main.cpp", Type: "blob", Size: 40},
		{Path: "rig.ino", Type: "blob", Size: 30},
		{Path: "include/arduino_secrets.h", Type: "blob", Size: 20},
		{Path: ".pio/libdeps/lib.cpp", Type: "blob", Size: 20},
		{Path: "tools/plot.py", Type: "blob", Size: 10},
		{Path: "README.md", Type: "blob", Size: 10},
		{Path: "src", Type: "tree"},
	}
	fake.files["src/main.cpp"] = "const int TRIG_PIN = 5;\n"
	fake.files["rig.ino"] = "void setup() { pinMode(TRIG_PIN, OUTPUT); }\n"
	fake.files["include/arduino_secrets.h"] = "#define WIFI_PASS \"hunter2\"\n"

	bundle, err := client.CollectSource(context.Background(), 7, "octo/rig", "abc123")
	if err != nil {
		t.Fatalf("CollectSource() error = %v", err)
	}
	if got := strings.Join(bundle.Files, ","); got != "rig.ino,src/main.cpp" {
		t.Fatalf("files = %s, want sketch first then src", got)
	}
	if bundle.Skipped != 3 {
		t.Fatalf("skipped = %d, want 3 (secret, dependency, python)", bundle.Skipped)
	}
	if bundle.Code.Language != "arduino-cpp" || strings.Contains(bundle.Code.Text, "hunter2") || !strings.Contains(bundle.Code.Text, "// ===== src/main.cpp =====") {
		t.Fatalf("code = %#v", bundle.Code)
	}
}

func TestCollectSourceWithoutSourceFiles(t *testing.T) {
	client, fake := newTestClient(t)
	fake.tree = []TreeEntry{{Path: "README.md", Type: "blob", Size: 10}}
	if _, err := client.CollectSource(context.Background(), 7, "octo/rig", "abc123"); err != ErrNoSource {
		t.Fatalf("CollectSource() error = %v, want ErrNoSource", err)
	}
}

func TestVerifyUserInstallation(t *testing.T) {
	client, fake := newTestClient(t)
	fake.userInstall = 7
	installation, err := client.VerifyUserInstallation(context.Background(), "good-code", 7)
	if err != nil || installation.Account.Login != "octo" {
		t.Fatalf("VerifyUserInstallation() = %#v, %v", installation, err)
	}
	if _, err := client.VerifyUserInstallation(context.Background(), "good-code", 8); err != ErrNotFound {
		t.Fatalf("someone else's installation: error = %v, want ErrNotFound", err)
	}
	if _, err := client.VerifyUserInstallation(context.Background(), "bad-code", 7); err == nil {
		t.Fatal("a rejected code must fail")
	}
}

func TestStateIsBoundToOwner(t *testing.T) {
	client, _ := newTestClient(t)
	state := client.SignState("owner-a")
	if !client.VerifyState(state, "owner-a") {
		t.Fatal("state should verify for the owner who started the install")
	}
	if client.VerifyState(state, "owner-b") || client.VerifyState(state+"0", "owner-a") || client.VerifyState("", "") {
		t.Fatal("state must not verify for another owner or when tampered")
	}
	other, _ := newTestClient(t)
	if other.VerifyState(state, "owner-a") {
		t.Fatal("state from another process key must not verify")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	client, _ := newTestClient(t)
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte("hook"))
	mac.Write(body)
	good := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !client.VerifyWebhookSignature(body, good) {
		t.Fatal("valid signature rejected")
	}
	if client.VerifyWebhookSignature([]byte(`{"ref":"refs/heads/evil"}`), good) || client.VerifyWebhookSignature(body, "sha1=abc") || client.VerifyWebhookSignature(body, "") {
		t.Fatal("invalid signature accepted")
	}
}

func TestConfigFromEnv(t *testing.T) {
	for _, name := range []string{"GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY", "GITHUB_APP_PRIVATE_KEY_PATH", "GITHUB_APP_SLUG", "GITHUB_APP_CLIENT_ID", "GITHUB_APP_CLIENT_SECRET", "GITHUB_WEBHOOK_SECRET"} {
		t.Setenv(name, "")
	}
	if _, ok, err := ConfigFromEnv(); ok || err != nil {
		t.Fatalf("unconfigured: ok=%v err=%v, want off without error", ok, err)
	}
	t.Setenv("GITHUB_APP_ID", "42")
	if _, ok, err := ConfigFromEnv(); ok || err == nil {
		t.Fatal("partial configuration must be an error")
	}
	t.Setenv("GITHUB_APP_SLUG", "https://github.com/settings/apps/reweird")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "unused")
	t.Setenv("GITHUB_APP_CLIENT_ID", "cid")
	t.Setenv("GITHUB_APP_CLIENT_SECRET", "secret")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "hook")
	if _, _, err := ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "GITHUB_APP_SLUG") {
		t.Fatalf("a URL as the slug must be rejected, got %v", err)
	}
}
