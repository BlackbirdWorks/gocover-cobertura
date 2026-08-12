package testfixtures

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteToTempDir(t *testing.T) {
	tmpDir := WriteToTempDir(t)
	
	info, err := os.Stat(tmpDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())

	// Check if a known file from embed was written
	func1Path := filepath.Join(tmpDir, "testdata", "func1.go")
	info, err = os.Stat(func1Path)
	require.NoError(t, err)
	require.False(t, info.IsDir())
}
