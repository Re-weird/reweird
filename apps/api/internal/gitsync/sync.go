package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/re-weird/reweird/apps/api/internal/reports"
)

var validID = regexp.MustCompile(`^test-[a-f0-9]{32}$`)

type Config struct {
	Repo       string
	Enabled    bool
	AutoCommit bool
	AutoPush   bool
}

func FromEnvironment() Config {
	return Config{Repo: os.Getenv("REWEIRD_GIT_REPO"), Enabled: os.Getenv("REWEIRD_GIT_SYNC_ENABLED") == "true", AutoCommit: os.Getenv("REWEIRD_AUTO_COMMIT_REPORTS") == "true", AutoPush: os.Getenv("REWEIRD_AUTO_PUSH_REPORTS") == "true"}
}

type Artifact struct {
	Path  string `json:"path"`
	Bytes int    `json:"bytes"`
}
type Preview struct {
	Enabled        bool       `json:"enabled"`
	RepoAvailable  bool       `json:"repo_available"`
	Files          []Artifact `json:"files"`
	SecretScan     string     `json:"secret_scan"`
	CommitAllowed  bool       `json:"commit_allowed"`
	PushConfigured bool       `json:"push_configured"`
	Detail         string     `json:"detail,omitempty"`
}

func RelativePaths(id string) (string, string, error) {
	if !validID.MatchString(id) {
		return "", "", errors.New("invalid diagnostic ID")
	}
	return filepath.Join(".reweird", "diagnostics", id+".json"), filepath.Join(".reweird", "reports", id+".md"), nil
}

func Artifacts(report reports.DetailedReport) ([]byte, []byte, error) {
	if _, _, err := RelativePaths(strings.TrimPrefix(report.ReportID, "report-")); err != nil {
		return nil, nil, err
	}
	jsonData, err := reports.JSON(report)
	if err != nil {
		return nil, nil, err
	}
	markdown := []byte(reports.Markdown(report))
	if len(jsonData) > reports.DetailedReportLimit() || len(markdown) > reports.DetailedReportLimit() {
		return nil, nil, errors.New("generated artifact exceeds 1 MiB")
	}
	return jsonData, markdown, nil
}

func SecretCount(report reports.DetailedReport, jsonData, markdown []byte) int {
	count := report.SecurityRedactionCount + reports.ScanSecrets(string(jsonData)) + reports.ScanSecrets(string(markdown))
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || len(parts[1]) < 8 {
			continue
		}
		key := strings.ToUpper(parts[0])
		if !strings.Contains(key, "KEY") && !strings.Contains(key, "TOKEN") && !strings.Contains(key, "SECRET") && !strings.Contains(key, "PASSWORD") && !strings.Contains(key, "CREDENTIAL") {
			continue
		}
		if strings.Contains(string(jsonData), parts[1]) || strings.Contains(string(markdown), parts[1]) {
			count++
		}
	}
	return count
}

func inspectRepo(repo string) (string, error) {
	if repo == "" {
		return "", errors.New("no Git repository configured")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("configured Git root must be a real directory")
	}
	// Resolve symlinks before comparing: on macOS the default TMPDIR sits
	// under /var, which is itself a symlink to /private/var, so git's
	// symlink-resolved --show-toplevel would otherwise never match abs.
	resolvedAbs, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	actual, err := git(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	actualAbs, err := filepath.Abs(strings.TrimSpace(actual))
	if err != nil || !strings.EqualFold(filepath.Clean(actualAbs), filepath.Clean(resolvedAbs)) {
		return "", errors.New("configured path must be the Git worktree root")
	}
	return abs, nil
}

func git(repo string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}
	return string(output), nil
}

func safeTargets(root string, paths ...string) error {
	for _, relative := range paths {
		path := filepath.Join(root, relative)
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(root)+string(filepath.Separator)) {
			return errors.New("artifact path escapes repository")
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("generated artifact already exists: %s", relative)
		} else if !os.IsNotExist(err) {
			return err
		}
		for directory := filepath.Dir(path); directory != root; directory = filepath.Dir(directory) {
			info, err := os.Lstat(directory)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("artifact parent must be a real directory")
			}
		}
	}
	return nil
}

func BuildPreview(config Config, report reports.DetailedReport) (Preview, error) {
	jsonPath, markdownPath, err := RelativePaths(strings.TrimPrefix(report.ReportID, "report-"))
	if err != nil {
		return Preview{}, err
	}
	jsonData, markdown, err := Artifacts(report)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{Enabled: config.Enabled, Files: []Artifact{{Path: filepath.ToSlash(jsonPath), Bytes: len(jsonData)}, {Path: filepath.ToSlash(markdownPath), Bytes: len(markdown)}}, SecretScan: "clear", PushConfigured: config.AutoPush}
	if SecretCount(report, jsonData, markdown) > 0 {
		preview.SecretScan = "blocked"
		preview.Detail = "Potential secret detected. Git synchronization blocked until resolved."
	}
	root, err := inspectRepo(config.Repo)
	if err != nil {
		preview.Detail = "No usable Git repository is configured."
		return preview, nil
	}
	preview.RepoAvailable = true
	if err := safeTargets(root, jsonPath, markdownPath); err != nil {
		preview.Detail = err.Error()
		return preview, nil
	}
	preview.CommitAllowed = config.Enabled && preview.SecretScan == "clear"
	if !config.Enabled && preview.Detail == "" {
		preview.Detail = "Git synchronization is disabled. Report remains local."
	}
	return preview, nil
}

// Commit only the two new generated paths. Other working-tree and index changes
// are never staged or included. Push additionally requires explicit approval.
func Commit(config Config, report reports.DetailedReport, push bool) (string, error) {
	preview, err := BuildPreview(config, report)
	if err != nil {
		return "", err
	}
	if !preview.CommitAllowed {
		return "", errors.New(preview.Detail)
	}
	if push && !config.AutoPush {
		return "", errors.New("push is not configured")
	}
	root, err := inspectRepo(config.Repo)
	if err != nil {
		return "", err
	}
	workflowID := strings.TrimPrefix(report.ReportID, "report-")
	jsonPath, markdownPath, _ := RelativePaths(workflowID)
	jsonData, markdown, err := Artifacts(report)
	if err != nil {
		return "", err
	}
	for _, relative := range []string{jsonPath, markdownPath} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0755); err != nil {
			return "", err
		}
	}
	if err := safeTargets(root, jsonPath, markdownPath); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, jsonPath), jsonData, 0600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, markdownPath), markdown, 0600); err != nil {
		return "", err
	}
	if _, err := git(root, "add", "--", jsonPath, markdownPath); err != nil {
		return "", err
	}
	if _, err := git(root, "commit", "--only", "-m", "ReWeird: add diagnostic report "+workflowID, "--", jsonPath, markdownPath); err != nil {
		return "", err
	}
	commit, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if push {
		branch, err := git(root, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return strings.TrimSpace(commit), err
		}
		if _, err := git(root, "push", "origin", "HEAD:refs/heads/"+strings.TrimSpace(branch)); err != nil {
			return strings.TrimSpace(commit), err
		}
	}
	return strings.TrimSpace(commit), nil
}
