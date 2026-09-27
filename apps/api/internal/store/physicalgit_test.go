package store

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

func TestPhysicalCommitSequenceIncrementsPerProject(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "physicalgit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	first, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-1111", ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-2222", ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Sequence != 1 || first.DisplayID != "HW-001" {
		t.Fatalf("first commit = %+v", first)
	}
	if second.Sequence != 2 || second.DisplayID != "HW-002" {
		t.Fatalf("second commit = %+v", second)
	}
	if first.CreatedAtMS == 0 || second.CreatedAtMS == 0 {
		t.Fatalf("expected created_at_ms to be stamped: %+v %+v", first, second)
	}

	// A different project has its own independent counter, starting at 1 again.
	otherProject, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-3333", ProjectID: "project-b"})
	if err != nil {
		t.Fatal(err)
	}
	if otherProject.Sequence != 1 || otherProject.DisplayID != "HW-001" {
		t.Fatalf("other project commit = %+v", otherProject)
	}
}

func TestPhysicalCommitDisplayIDWidensPastThreeDigits(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "physicalgit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	var last domain.PhysicalCommit
	for index := 0; index < 1000; index++ {
		commit, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: fmt.Sprintf("pcommit-%d", index), ProjectID: "project-a"})
		if err != nil {
			t.Fatal(err)
		}
		last = commit
	}
	if last.Sequence != 1000 || last.DisplayID != "HW-1000" {
		t.Fatalf("1000th commit = %+v, want display id HW-1000 (not truncated)", last)
	}
}

func TestPhysicalCommitGetFiltersByProject(t *testing.T) {
	repository, err := Open(filepath.Join(t.TempDir(), "physicalgit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	stored, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-aaaa", ProjectID: "project-a", Note: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	found, err := repository.GetPhysicalCommit("project-a", stored.ID)
	if err != nil || found == nil || found.Note != "hello" {
		t.Fatalf("get = %+v, %v", found, err)
	}

	// The same commit id under a different project must 404-equivalent (nil, nil),
	// never leaking that it exists elsewhere.
	crossProject, err := repository.GetPhysicalCommit("project-b", stored.ID)
	if err != nil || crossProject != nil {
		t.Fatalf("cross-project get = %+v, %v, want nil, nil", crossProject, err)
	}

	missing, err := repository.GetPhysicalCommit("project-a", "pcommit-does-not-exist")
	if err != nil || missing != nil {
		t.Fatalf("missing get = %+v, %v, want nil, nil", missing, err)
	}
}

func TestPhysicalCommitListOrdersNewestFirstAndPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "physicalgit.db")
	repository, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-1111", ProjectID: "project-a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.SavePhysicalCommit(domain.PhysicalCommit{ID: "pcommit-2222", ProjectID: "project-a"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.ListPhysicalCommits("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "pcommit-2222" || items[1].ID != "pcommit-1111" {
		t.Fatalf("list = %+v, want newest (pcommit-2222) first", items)
	}
}
