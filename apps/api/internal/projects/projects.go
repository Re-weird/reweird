package projects

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/re-weird/reweird/apps/api/internal/domain"
)

const (
	MaxImageBytes = 5 * 1024 * 1024
	MaxCodeBytes  = 512 * 1024
)

var safeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

func NewID(name string) (string, error) {
	suffix := make([]byte, 5)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	base := slug(name)
	if len(base) > 48 {
		base = strings.Trim(base[:48], "-")
	}
	if len(base) < 3 {
		base = "project"
	}
	return base + "-" + hex.EncodeToString(suffix), nil
}

func Validate(project domain.Project) error {
	if !safeIDPattern.MatchString(project.ID) {
		return errors.New("project id is invalid")
	}
	if strings.TrimSpace(project.Name) == "" || len(project.Name) > 120 {
		return errors.New("name is required and must not exceed 120 characters")
	}
	if len(project.Description) > 2000 {
		return errors.New("description must not exceed 2000 characters")
	}
	if strings.TrimSpace(project.Controller) == "" || len(project.Controller) > 80 {
		return errors.New("controller is required and must not exceed 80 characters")
	}
	if project.LogicVoltage <= 0 || project.LogicVoltage > 5.5 {
		return errors.New("logic_voltage must be greater than 0 and at most 5.5 V")
	}
	return nil
}

func SaveImage(uploadRoot, projectID, filename string, reader io.Reader) (*domain.ProjectMedia, error) {
	if !safeIDPattern.MatchString(projectID) {
		return nil, errors.New("invalid project id")
	}
	payload, err := readLimited(reader, MaxImageBytes)
	if err != nil {
		return nil, err
	}
	contentType := http.DetectContentType(payload)
	extension := ""
	switch contentType {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	default:
		return nil, fmt.Errorf("unsupported image type %q; use PNG or JPEG", contentType)
	}
	original := SanitizeFilename(filename)
	if original == "" {
		original = "hardware" + extension
	}
	hash := sha256.Sum256(payload)
	hashText := hex.EncodeToString(hash[:])
	root, err := filepath.Abs(uploadRoot)
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(root, projectID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create project upload directory: %w", err)
	}
	storedName := "hardware-" + hashText[:16] + extension
	target := filepath.Join(directory, storedName)
	if err := ensureWithin(root, target); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("store project image: %w", err)
	}
	if err == nil {
		if _, writeErr := file.Write(payload); writeErr != nil {
			_ = file.Close()
			return nil, fmt.Errorf("store project image: %w", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return nil, closeErr
		}
	}
	reference, err := filepath.Rel(root, target)
	if err != nil {
		return nil, err
	}
	return &domain.ProjectMedia{StorageRef: filepath.ToSlash(reference), OriginalFilename: original, ContentType: contentType, SizeBytes: int64(len(payload)), SHA256: hashText}, nil
}

func ResolveImage(uploadRoot, reference string) (string, error) {
	root, err := filepath.Abs(uploadRoot)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(reference)))
	if err != nil {
		return "", err
	}
	if err := ensureWithin(root, target); err != nil {
		return "", err
	}
	return target, nil
}

func ReadCode(filename string, reader io.Reader) (*domain.ProjectCode, error) {
	filename = SanitizeFilename(filename)
	if filename == "" {
		return nil, errors.New("code filename is required")
	}
	lower := strings.ToLower(filename)
	if strings.HasPrefix(lower, ".env") || hasExtension(lower, ".pem", ".key", ".p12", ".pfx") {
		return nil, errors.New("secret or credential files cannot be uploaded")
	}
	if !hasExtension(lower, ".ino", ".cpp", ".h", ".hpp", ".c", ".py", ".txt") {
		return nil, errors.New("unsupported code file; use .ino, .cpp, .h, .hpp, .c, .py, or .txt")
	}
	payload, err := readLimited(reader, MaxCodeBytes)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 || looksBinary(payload) {
		return nil, errors.New("uploaded code must be UTF-8 text, not a binary file")
	}
	hash := sha256.Sum256(payload)
	return &domain.ProjectCode{Filename: filename, Language: languageFor(filename), Text: string(payload), SizeBytes: int64(len(payload)), SHA256: hex.EncodeToString(hash[:])}, nil
}

func SanitizeFilename(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	value = strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("._-", character) {
			return character
		}
		return '_'
	}, value)
	value = strings.Trim(value, " .")
	if len(value) > 120 {
		extension := filepath.Ext(value)
		value = strings.TrimSuffix(value, extension)
		value = value[:min(100, len(value))] + extension
	}
	return value
}

func readLimited(reader io.Reader, maximum int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, errors.New("uploaded file is empty")
	}
	if int64(len(payload)) > maximum {
		return nil, fmt.Errorf("uploaded file exceeds the %d byte limit", maximum)
	}
	return payload, nil
}

func looksBinary(payload []byte) bool {
	controls := 0
	for _, value := range payload {
		if value < 0x09 || value > 0x0d && value < 0x20 {
			controls++
		}
	}
	return controls > len(payload)/100+1
}

func ensureWithin(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("upload path escapes the configured storage root")
	}
	return nil
}

func hasExtension(filename string, extensions ...string) bool {
	extension := strings.ToLower(filepath.Ext(filename))
	for _, allowed := range extensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

func languageFor(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".ino":
		return "arduino-cpp"
	case ".cpp", ".h", ".hpp":
		return "cpp"
	case ".c":
		return "c"
	case ".py":
		return "python"
	default:
		return "text"
	}
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
