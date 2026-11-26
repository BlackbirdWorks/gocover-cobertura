// Package testfixtures embeds sample Go files and coverage profiles for tests.
package testfixtures

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blackbirdworks/gocover-cobertura/pkg/errs"
)

// FS holds the embedded test fixture files.
//
//go:embed testdata/*
var FS embed.FS //nolint:varnamelen // FS is an exported Go embed.FS identifier.

var errReadFixture = errors.New("failed to read embedded fixture")

// WriteToTempDir extracts the embedded testdata into a temporary directory
// and returns the path to that temporary directory. This is particularly
// useful for tests that require a physical file system, such as integration tests.
func WriteToTempDir(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()

	err := fs.WalkDir(FS, "testdata", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		destPath := filepath.Join(tempDir, path)

		if entry.IsDir() {
			return os.MkdirAll(destPath, 0o750)
		}

		data, readErr := fs.ReadFile(FS, path)
		if readErr != nil {
			return errs.Wrapf(readErr, errReadFixture, "%q", path)
		}

		return os.WriteFile(destPath, data, 0o600)
	})
	require.NoError(t, err, "failed to write embedded testdata to temp dir")

	return tempDir
}
