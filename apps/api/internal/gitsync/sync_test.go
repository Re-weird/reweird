package gitsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/reports"
)

const testID = "test-0123456789abcdef0123456789abcdef"

func testRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Test Human"}, {"config", "user.email", "test@example.invalid"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("existing project"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "Initial project"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	return root
}

func TestPreviewDefaultDisabledAndKeepLocal(t *testing.T) {
	root := testRepository(t)
	report := reports.DetailedReport{ReportID: "report-" + testID, Summary: "Measured evidence only"}
	preview, err := BuildPreview(Config{Repo: root}, report)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CommitAllowed || !preview.RepoAvailable || len(preview.Files) != 2 || preview.SecretScan != "clear" {
		t.Fatalf("preview %+v", preview)
	}
	if _, err := Commit(Config{Repo: root}, report, false); err == nil {
		t.Fatal("disabled sync committed")
	}
	if _, err := os.Stat(filepath.Join(root, ".reweird")); !os.IsNotExist(err) {
		t.Fatal("preview/keep local wrote files")
	}
}

func TestSecretBlocksAndCommitPreservesAuthorship(t *testing.T) {
	root := testRepository(t)
	report := reports.DetailedReport{ReportID: "report-" + testID, Summary: "Measured evidence only", SecurityRedactionCount: 1}
	preview, err := BuildPreview(Config{Repo: root, Enabled: true}, report)
	if err != nil || preview.SecretScan != "blocked" || preview.CommitAllowed {
		t.Fatalf("secret preview %+v %v", preview, err)
	}
	if _, err := Commit(Config{Repo: root, Enabled: true}, report, false); err == nil {
		t.Fatal("secret committed")
	}
	report.SecurityRedactionCount = 0
	if _, err := Commit(Config{Repo: root, Enabled: true}, report, true); err == nil {
		t.Fatal("unconfigured push accepted")
	}
	commit, err := Commit(Config{Repo: root, Enabled: true}, report, false)
	if err != nil || len(commit) < 30 {
		t.Fatalf("commit %q: %v", commit, err)
	}
	command := exec.Command("git", "show", "-s", "--format=%an <%ae> %s", "HEAD")
	command.Dir = root
	output, err := command.Output()
	if err != nil || !strings.Contains(string(output), "Test Human <test@example.invalid> ReWeird: add diagnostic report "+testID) {
		t.Fatalf("attribution %q %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".reweird", "reports", testID+".md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(Config{Repo: root, Enabled: true}, report, false); err == nil {
		t.Fatal("existing artifact overwritten")
	}
}

func TestRejectsTraversal(t *testing.T) {
	if _, _, err := RelativePaths("../../secret"); err == nil {
		t.Fatal("traversal accepted")
	}
}
