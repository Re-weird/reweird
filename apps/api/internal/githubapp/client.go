// Package githubapp talks to GitHub as a GitHub App: it lists the repos a
// user granted to their installation, reads source files at a commit, and
// verifies push webhooks. Access is read-only (Contents + Metadata), and no
// GitHub user token is ever persisted.
package githubapp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrNotFound is returned when GitHub answers 404, which also covers a repo
// the installation has no access to.
var ErrNotFound = errors.New("github: not found")

type Config struct {
	AppID         int64
	Slug          string
	ClientID      string
	ClientSecret  string
	PrivateKey    *rsa.PrivateKey
	WebhookSecret string
	APIURL        string
	WebURL        string
}

// ConfigFromEnv reads the GitHub App settings. ok is false when the App
// isn't configured at all, which leaves the GitHub features switched off;
// a partially filled configuration is an error so it can't fail silently.
func ConfigFromEnv() (Config, bool, error) {
	config := Config{
		Slug:          strings.TrimSpace(os.Getenv("GITHUB_APP_SLUG")),
		ClientID:      strings.TrimSpace(os.Getenv("GITHUB_APP_CLIENT_ID")),
		ClientSecret:  strings.TrimSpace(os.Getenv("GITHUB_APP_CLIENT_SECRET")),
		WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		APIURL:        strings.TrimRight(envOr("GITHUB_API_URL", "https://api.github.com"), "/"),
		WebURL:        strings.TrimRight(envOr("GITHUB_WEB_URL", "https://github.com"), "/"),
	}
	rawID := strings.TrimSpace(os.Getenv("GITHUB_APP_ID"))
	keyPEM := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if path := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")); path != "" {
		payload, err := os.ReadFile(path)
		if err != nil {
			return Config{}, false, fmt.Errorf("read GITHUB_APP_PRIVATE_KEY_PATH: %w", err)
		}
		keyPEM = string(payload)
	}
	if rawID == "" && keyPEM == "" && config.Slug == "" && config.ClientID == "" {
		return Config{}, false, nil
	}
	missing := []string{}
	for name, value := range map[string]string{"GITHUB_APP_ID": rawID, "GITHUB_APP_PRIVATE_KEY(_PATH)": keyPEM, "GITHUB_APP_SLUG": config.Slug, "GITHUB_APP_CLIENT_ID": config.ClientID, "GITHUB_APP_CLIENT_SECRET": config.ClientSecret, "GITHUB_WEBHOOK_SECRET": config.WebhookSecret} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, false, fmt.Errorf("GitHub App is partially configured; missing %s", strings.Join(missing, ", "))
	}
	if strings.ContainsAny(config.Slug, "/:") {
		return Config{}, false, errors.New("GITHUB_APP_SLUG must be the App's URL name only (the <slug> in github.com/apps/<slug>), not a URL")
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		return Config{}, false, errors.New("GITHUB_APP_ID must be a positive integer")
	}
	config.AppID = id
	// .env files often hold the PEM on one line with literal \n escapes.
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(strings.ReplaceAll(keyPEM, `\n`, "\n")))
	if err != nil {
		return Config{}, false, fmt.Errorf("parse GitHub App private key: %w", err)
	}
	config.PrivateKey = key
	return config, true, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

type Client struct {
	config   Config
	http     *http.Client
	stateKey []byte

	tokenMu sync.Mutex
	tokens  map[int64]cachedToken
}

type cachedToken struct {
	value     string
	expiresAt time.Time
}

func New(config Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	stateKey := make([]byte, 32)
	if _, err := rand.Read(stateKey); err != nil {
		panic(fmt.Sprintf("githubapp: random state key: %v", err))
	}
	return &Client{config: config, http: httpClient, stateKey: stateKey, tokens: map[int64]cachedToken{}}
}

// InstallURL is where Settings sends the user to install (or reconfigure)
// the App. state is echoed back to the setup callback.
func (client *Client) InstallURL(state string) string {
	return fmt.Sprintf("%s/apps/%s/installations/new?state=%s", client.config.WebURL, url.PathEscape(client.config.Slug), url.QueryEscape(state))
}

// AuthorizeURL asks GitHub who the user is (skipped automatically once
// they've approved the App). Used when the App may already be installed,
// where the install page would not redirect back.
func (client *Client) AuthorizeURL(state string) string {
	return fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&state=%s", client.config.WebURL, url.QueryEscape(client.config.ClientID), url.QueryEscape(state))
}

// SignState binds an install round trip to the ReWeird user who started it,
// so a callback can't attach someone else's session to an installation.
// The key is per process, so a restart invalidates in-flight installs.
func (client *Client) SignState(ownerID string) string {
	expires := strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	body := base64.RawURLEncoding.EncodeToString([]byte(ownerID)) + "." + expires + "." + hex.EncodeToString(nonce)
	return body + "." + client.stateMAC(body)
}

func (client *Client) VerifyState(state, ownerID string) bool {
	index := strings.LastIndex(state, ".")
	if index <= 0 {
		return false
	}
	body, mac := state[:index], state[index+1:]
	if !hmac.Equal([]byte(mac), []byte(client.stateMAC(body))) {
		return false
	}
	parts := strings.Split(body, ".")
	if len(parts) != 3 {
		return false
	}
	owner, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || string(owner) != ownerID {
		return false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && time.Now().Unix() <= expires
}

func (client *Client) stateMAC(body string) string {
	mac := hmac.New(sha256.New, client.stateKey)
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhookSignature checks GitHub's X-Hub-Signature-256 header against
// the raw request body.
func (client *Client) VerifyWebhookSignature(body []byte, header string) bool {
	signature, ok := strings.CutPrefix(header, "sha256=")
	if !ok || client.config.WebhookSecret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(client.config.WebhookSecret))
	mac.Write(body)
	return hmac.Equal([]byte(signature), []byte(hex.EncodeToString(mac.Sum(nil))))
}

func (client *Client) appJWT() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		// Backdated to absorb clock drift, as GitHub recommends.
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
		Issuer:    strconv.FormatInt(client.config.AppID, 10),
	})
	return token.SignedString(client.config.PrivateKey)
}

func (client *Client) installationToken(ctx context.Context, installationID int64) (string, error) {
	client.tokenMu.Lock()
	defer client.tokenMu.Unlock()
	if cached, ok := client.tokens[installationID]; ok && time.Until(cached.expiresAt) > 2*time.Minute {
		return cached.value, nil
	}
	appToken, err := client.appJWT()
	if err != nil {
		return "", err
	}
	var response struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	if err := client.do(ctx, http.MethodPost, client.config.APIURL+path, "Bearer "+appToken, "", nil, &response); err != nil {
		return "", err
	}
	client.tokens[installationID] = cachedToken{value: response.Token, expiresAt: response.ExpiresAt}
	return response.Token, nil
}

// Forget drops a cached token, e.g. after GitHub reports an uninstall.
func (client *Client) Forget(installationID int64) {
	client.tokenMu.Lock()
	delete(client.tokens, installationID)
	client.tokenMu.Unlock()
}

type Installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"account"`
}

// UserGrant is who approved ReWeird on GitHub and which installations of
// this App that GitHub user can access.
type UserGrant struct {
	Login         string
	Installations []Installation
}

// UserInstallations exchanges the OAuth code GitHub sends back after
// authorization (or after an install that requests authorization) for a
// short-lived user token, reads who the user is, and lists the
// installations of this App they can access. The token is then discarded.
func (client *Client) UserInstallations(ctx context.Context, code string) (*UserGrant, error) {
	form := url.Values{"client_id": {client.config.ClientID}, "client_secret": {client.config.ClientSecret}, "code": {code}}
	var exchanged struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := client.do(ctx, http.MethodPost, client.config.WebURL+"/login/oauth/access_token", "", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &exchanged); err != nil {
		return nil, err
	}
	if exchanged.AccessToken == "" {
		return nil, fmt.Errorf("github: authorization code rejected (%s)", exchanged.Error)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := client.do(ctx, http.MethodGet, client.config.APIURL+"/user", "Bearer "+exchanged.AccessToken, "", nil, &user); err != nil {
		return nil, err
	}
	grant := &UserGrant{Login: user.Login, Installations: []Installation{}}
	for page := 1; page <= 10; page++ {
		var listing struct {
			Installations []Installation `json:"installations"`
		}
		endpoint := fmt.Sprintf("%s/user/installations?per_page=100&page=%d", client.config.APIURL, page)
		if err := client.do(ctx, http.MethodGet, endpoint, "Bearer "+exchanged.AccessToken, "", nil, &listing); err != nil {
			return nil, err
		}
		grant.Installations = append(grant.Installations, listing.Installations...)
		if len(listing.Installations) < 100 {
			break
		}
	}
	return grant, nil
}

// VerifyUserInstallation confirms, via the OAuth code, that the user can
// access installationID, and returns who they are.
func (client *Client) VerifyUserInstallation(ctx context.Context, code string, installationID int64) (*Installation, string, error) {
	grant, err := client.UserInstallations(ctx, code)
	if err != nil {
		return nil, "", err
	}
	for _, installation := range grant.Installations {
		if installation.ID == installationID {
			return &installation, grant.Login, nil
		}
	}
	return nil, grant.Login, ErrNotFound
}

type Repository struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	DefaultBranch string    `json:"default_branch"`
	HTMLURL       string    `json:"html_url"`
	Language      string    `json:"language"`
	PushedAt      time.Time `json:"pushed_at"`
	Archived      bool      `json:"archived"`
}

// ListRepositories returns every repo granted to the installation, capped
// at 1,000 so a huge organisation can't stall the request.
func (client *Client) ListRepositories(ctx context.Context, installationID int64) ([]Repository, error) {
	token, err := client.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	repositories := []Repository{}
	for page := 1; page <= 10; page++ {
		var listing struct {
			Repositories []Repository `json:"repositories"`
		}
		endpoint := fmt.Sprintf("%s/installation/repositories?per_page=100&page=%d", client.config.APIURL, page)
		if err := client.do(ctx, http.MethodGet, endpoint, "token "+token, "", nil, &listing); err != nil {
			return nil, err
		}
		repositories = append(repositories, listing.Repositories...)
		if len(listing.Repositories) < 100 {
			break
		}
	}
	return repositories, nil
}

// GetRepository returns ErrNotFound when the installation can't see the
// repo, which is how a create request for someone else's repo is refused.
func (client *Client) GetRepository(ctx context.Context, installationID int64, fullName string) (*Repository, error) {
	token, err := client.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	var repository Repository
	if err := client.do(ctx, http.MethodGet, client.config.APIURL+"/repos/"+repoPath(fullName), "token "+token, "", nil, &repository); err != nil {
		return nil, err
	}
	return &repository, nil
}

type Commit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

// Commit resolves a branch name or SHA to its commit.
func (client *Client) Commit(ctx context.Context, installationID int64, fullName, ref string) (*Commit, error) {
	token, err := client.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	var commit Commit
	if err := client.do(ctx, http.MethodGet, client.config.APIURL+"/repos/"+repoPath(fullName)+"/commits/"+url.PathEscape(ref), "token "+token, "", nil, &commit); err != nil {
		return nil, err
	}
	return &commit, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

func (client *Client) Tree(ctx context.Context, installationID int64, fullName, sha string) ([]TreeEntry, bool, error) {
	token, err := client.installationToken(ctx, installationID)
	if err != nil {
		return nil, false, err
	}
	var tree struct {
		Tree      []TreeEntry `json:"tree"`
		Truncated bool        `json:"truncated"`
	}
	endpoint := client.config.APIURL + "/repos/" + repoPath(fullName) + "/git/trees/" + url.PathEscape(sha) + "?recursive=1"
	if err := client.do(ctx, http.MethodGet, endpoint, "token "+token, "", nil, &tree); err != nil {
		return nil, false, err
	}
	return tree.Tree, tree.Truncated, nil
}

// RawFile reads one file at a commit, refusing anything over limit bytes.
func (client *Client) RawFile(ctx context.Context, installationID int64, fullName, path, sha string, limit int64) ([]byte, error) {
	token, err := client.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	segments := strings.Split(path, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	endpoint := client.config.APIURL + "/repos/" + repoPath(fullName) + "/contents/" + strings.Join(segments, "/") + "?ref=" + url.QueryEscape(sha)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "token "+token)
	request.Header.Set("Accept", "application/vnd.github.raw+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("github: %s returned %d", path, response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("github: %s exceeds %d bytes", path, limit)
	}
	return payload, nil
}

func repoPath(fullName string) string {
	owner, name, _ := strings.Cut(fullName, "/")
	return url.PathEscape(owner) + "/" + url.PathEscape(name)
}

func (client *Client) do(ctx context.Context, method, endpoint, authorization, contentType string, body io.Reader, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode >= 300 {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(payload, &failure)
		return fmt.Errorf("github: %s %s returned %d: %s", method, request.URL.Path, response.StatusCode, failure.Message)
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(payload, target)
}
