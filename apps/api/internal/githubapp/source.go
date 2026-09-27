package githubapp

import (
	"bytes"
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/re-weird/reweird/apps/api/internal/domain"
	"github.com/re-weird/reweird/apps/api/internal/projects"
)

const (
	maxSourceFiles   = 150
	maxSourceFileLen = 128 * 1024
)

var cFamily = map[string]bool{".ino": true, ".cpp": true, ".c": true, ".h": true, ".hpp": true}

// Directories that hold dependencies, build output, or CI config rather than
// the user's firmware.
var skippedDirs = map[string]bool{
	".git": true, ".github": true, ".pio": true, ".vscode": true, "node_modules": true, "build": true,
	"dist": true, "vendor": true, "third_party": true, ".venv": true, "venv": true, "__pycache__": true,
}

// ErrNoSource means the commit has no file the code analyzer understands.
var ErrNoSource = errors.New("no .ino, .cpp, .c, .h, .hpp, or .py source files were found on this commit")

// SourceBundle is the repo's firmware at one commit, flattened into the
// single code input the existing analyzer accepts.
type SourceBundle struct {
	Code    *domain.ProjectCode
	Files   []string
	Skipped int
}

type candidate struct {
	path string
	size int64
}

// CollectSource picks the firmware files at sha and concatenates them, each
// under a path header. Files that look like credentials (arduino_secrets.h,
// .env, keys) are never read. The analyzer parses one language, so only the
// family with the most source bytes (C/C++ or Python) is included.
func (client *Client) CollectSource(ctx context.Context, installationID int64, fullName, sha string) (*SourceBundle, error) {
	entries, truncated, err := client.Tree(ctx, installationID, fullName, sha)
	if err != nil {
		return nil, err
	}
	var cFiles, pyFiles []candidate
	var cBytes, pyBytes int64
	skipped := 0
	if truncated {
		skipped++
	}
	for _, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		extension := strings.ToLower(path.Ext(entry.Path))
		if !cFamily[extension] && extension != ".py" {
			continue
		}
		if inSkippedDir(entry.Path) || looksSecret(entry.Path) || entry.Size > maxSourceFileLen {
			skipped++
			continue
		}
		if extension == ".py" {
			pyFiles, pyBytes = append(pyFiles, candidate{entry.Path, entry.Size}), pyBytes+entry.Size
		} else {
			cFiles, cBytes = append(cFiles, candidate{entry.Path, entry.Size}), cBytes+entry.Size
		}
	}
	chosen, filename, comment := cFiles, "repository.cpp", "//"
	if pyBytes > cBytes {
		chosen, filename, comment = pyFiles, "repository.py", "#"
		skipped += len(cFiles)
	} else {
		skipped += len(pyFiles)
		for _, file := range cFiles {
			if strings.EqualFold(path.Ext(file.path), ".ino") {
				filename = "repository.ino"
				break
			}
		}
	}
	if len(chosen) == 0 {
		return nil, ErrNoSource
	}
	sort.SliceStable(chosen, func(i, j int) bool {
		return rank(chosen[i].path) < rank(chosen[j].path) || rank(chosen[i].path) == rank(chosen[j].path) && chosen[i].path < chosen[j].path
	})

	budget := int64(projects.MaxCodeBytes)
	var combined bytes.Buffer
	files := []string{}
	for _, file := range chosen {
		if len(files) == maxSourceFiles {
			skipped++
			continue
		}
		header := comment + " ===== " + file.path + " =====\n"
		if int64(combined.Len()+len(header))+file.size+1 > budget {
			skipped++
			continue
		}
		payload, err := client.RawFile(ctx, installationID, fullName, file.path, sha, maxSourceFileLen)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 || int64(combined.Len()+len(header)+len(payload)+1) > budget {
			skipped++
			continue
		}
		combined.WriteString(header)
		combined.Write(payload)
		combined.WriteByte('\n')
		files = append(files, file.path)
	}
	if len(files) == 0 {
		return nil, ErrNoSource
	}
	code, err := projects.ReadCode(filename, &combined)
	if err != nil {
		return nil, err
	}
	return &SourceBundle{Code: code, Files: files, Skipped: skipped}, nil
}

// rank puts sketch files and src/ first so they survive the size budget.
func rank(filePath string) int {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".ino"):
		return 0
	case strings.HasPrefix(lower, "src/") || !strings.Contains(lower, "/"):
		return 1
	case strings.HasPrefix(lower, "include/") || strings.HasPrefix(lower, "lib/"):
		return 3
	default:
		return 2
	}
}

func inSkippedDir(filePath string) bool {
	for _, segment := range strings.Split(filePath, "/")[:strings.Count(filePath, "/")] {
		if skippedDirs[strings.ToLower(segment)] {
			return true
		}
	}
	return false
}

func looksSecret(filePath string) bool {
	name := strings.ToLower(path.Base(filePath))
	for _, marker := range []string{"secret", "credential", "password", "passwd", "token", "apikey", "api_key", ".env"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}
