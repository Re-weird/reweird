package projects

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCodeValidation(t *testing.T) {
	code, err := ReadCode("../distance.ino", bytes.NewBufferString("#define TRIG 5\n"))
	if err != nil || code.Filename != "distance.ino" || code.Language != "arduino-cpp" {
		t.Fatalf("ReadCode() = %#v, %v", code, err)
	}
	if _, err := ReadCode("firmware.bin", bytes.NewReader([]byte{0, 1, 2})); err == nil {
		t.Fatal("binary upload was accepted")
	}
	if _, err := ReadCode(".env", bytes.NewBufferString("SECRET=value")); err == nil {
		t.Fatal("secret upload was accepted")
	}
}

func TestProjectImageCountAggregateLimitAndCleanup(t *testing.T) {
	root := t.TempDir()
	png := func(marker byte) []byte {
		return append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', marker}, make([]byte, 504)...)
	}
	first, err := SaveImage(root, "test-project", "first.png", 0, bytes.NewReader(png(1)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := SaveImage(root, "test-project", "second.png", 0, bytes.NewReader(png(2)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImage(root, "test-project", "third.png", 0, bytes.NewReader(png(3))); !errors.Is(err, ErrStorageLimit) {
		t.Fatalf("third image error = %v, want storage limit", err)
	}
	if err := PruneObsoleteImages(root, "test-project", second.StorageRef); err != nil {
		t.Fatal(err)
	}
	oldPath, err := ResolveImage(root, first.StorageRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete image still exists: %v", err)
	}
	if _, err := SaveImage(root, "test-project", "third.png", 0, bytes.NewReader(png(3))); err != nil {
		t.Fatalf("replacement after cleanup: %v", err)
	}
	if _, err := SaveImage(root, "another-project", "large.png", MaxProjectStoredBytes, bytes.NewReader(png(4))); !errors.Is(err, ErrStorageLimit) {
		t.Fatalf("aggregate storage error = %v, want storage limit", err)
	}
}

func TestImageValidationAndSafeStorage(t *testing.T) {
	root := t.TempDir()
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 504)...)
	media, err := SaveImage(root, "test-project", "../../photo.png", 0, bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	path, err := ResolveImage(root, media.StorageRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != filepath.Join(root, "test-project") {
		t.Fatalf("stored outside project directory: %s", path)
	}
	if _, err := SaveImage(root, "test-project", "bad.txt", 0, bytes.NewBufferString("not an image")); err == nil {
		t.Fatal("non-image upload was accepted")
	}
}
