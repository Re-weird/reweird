package projects

import (
	"bytes"
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

func TestImageValidationAndSafeStorage(t *testing.T) {
	root := t.TempDir()
	png := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, make([]byte, 504)...)
	media, err := SaveImage(root, "test-project", "../../photo.png", bytes.NewReader(png))
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
	if _, err := SaveImage(root, "test-project", "bad.txt", bytes.NewBufferString("not an image")); err == nil {
		t.Fatal("non-image upload was accepted")
	}
}
