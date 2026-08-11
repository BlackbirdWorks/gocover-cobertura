package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestCLIRun_File(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.xml")

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	cli := CLI{
		File:   "testdata/testdata_set.txt",
		Output: outFile,
	}

	err = cli.Run()
	require.NoError(t, err)

	assert.FileExists(t, outFile)
}

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestCLIRun_PatternGlob(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.xml")

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	cli := CLI{
		Pattern: "testdata/**/*.txt",
		Output:  outFile,
	}

	err = cli.Run()
	require.NoError(t, err)

	assert.FileExists(t, outFile)
}

func TestCLIRun_Error(t *testing.T) {
	t.Parallel()

	cli := CLI{
		File:   "nonexistent-file.txt",
		Output: "-",
	}

	err := cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open input file")
}

func TestCLIRun_PatternNoMatch(t *testing.T) {
	t.Parallel()

	cli := CLI{
		Pattern: "nonexistent-dir/**/*.out",
		Output:  "-",
	}

	err := cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no files matched pattern")
}
