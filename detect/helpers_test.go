package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExistsTrue(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("hi"), 0o644)

	if !FileExists(path) {
		t.Error("expected FileExists to return true for existing file")
	}
}

func TestFileExistsFalse(t *testing.T) {
	if FileExists("/nonexistent/path/to/file.txt") {
		t.Error("expected FileExists to return false for nonexistent file")
	}
}

func TestFileExistsDirectory(t *testing.T) {
	tmp := t.TempDir()
	if !FileExists(tmp) {
		t.Error("expected FileExists to return true for directory")
	}
}
