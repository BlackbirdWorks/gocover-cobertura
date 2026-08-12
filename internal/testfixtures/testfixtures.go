package testfixtures

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/*
var FS embed.FS

// WriteToTempDir extracts the embedded testdata into a temporary directory
// and returns the path to that temporary directory. This is particularly
// useful for tests that require a physical file system, such as integration tests.
func WriteToTempDir(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()

	err := fs.WalkDir(FS, "testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		destPath := filepath.Join(tempDir, path)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0750)
		}

		data, err := fs.ReadFile(FS, path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0600)
	})

	if err != nil {
		t.Fatalf("failed to write embedded testdata to temp dir: %v", err)
	}

	return tempDir
}
