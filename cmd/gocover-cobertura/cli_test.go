package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestClose = errors.New("close error")

type failCloser struct{}

func (failCloser) Close() error {
	return errTestClose
}

func TestMultiReadCloser_CloseError(t *testing.T) {
	t.Parallel()

	mc := &multiReadCloser{
		Reader:  os.Stdin,
		closers: []io.Closer{},
	}

	err := mc.Close()
	require.NoError(t, err)

	mcFail := &multiReadCloser{
		Reader:  os.Stdin,
		closers: []io.Closer{failCloser{}},
	}

	errFail := mcFail.Close()
	require.Error(t, errFail)
	assert.Contains(t, errFail.Error(), "close error")
}

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
func TestCLIRun_StdinStdout(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	cli := CLI{
		File:   "-",
		Output: "-",
	}

	err = cli.Run()
	require.NoError(t, err)
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

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestCLIRun_PatternDoubleStarOnly(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.xml")

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	cli := CLI{
		Pattern: "**/testdata_set.txt",
		Output:  outFile,
	}

	err = cli.Run()
	require.NoError(t, err)

	assert.FileExists(t, outFile)
}

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestCLIRun_PatternNonRecursiveGlob(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.xml")

	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	cli := CLI{
		Pattern: "testdata/*.txt",
		Output:  outFile,
	}

	err = cli.Run()
	require.NoError(t, err)

	assert.FileExists(t, outFile)
}

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestCLIRun_ConvertFailed(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.txt")
	err := os.WriteFile(invalidFile, []byte("invalid data"), 0644)
	require.NoError(t, err)

	cli := CLI{
		File:   invalidFile,
		Output: "-",
	}

	err = cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversion failed")
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

func TestCLIRun_PatternBadSyntax(t *testing.T) {
	t.Parallel()

	cli := CLI{
		Pattern: "invalid[pattern",
		Output:  "-",
	}

	err := cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process pattern")

	_, globErr := findMatchingFiles("invalid[pattern")
	require.Error(t, globErr)
}

func TestCLIRun_OutputCreateError(t *testing.T) {
	t.Parallel()

	cli := CLI{
		File:   os.DevNull,
		Output: "/invalid-dir-path/output.xml",
	}

	err := cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create output file")
}

func TestCLIRun_PatternUnreadableFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "1_valid.out")
	err := os.WriteFile(validFile, []byte("mode: set\n"), 0644)
	require.NoError(t, err)

	unreadableFile := filepath.Join(tmpDir, "2_unreadable.out")
	err = os.WriteFile(unreadableFile, []byte("mode: set\n"), 0000)
	require.NoError(t, err)

	cli := CLI{
		Pattern: filepath.Join(tmpDir, "*.out"),
		Output:  "-",
	}

	err = cli.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open matched file")
}

func TestCLIRun_PatternWalkError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0000)
	require.NoError(t, err)

	_, walkErr := findMatchingFiles(tmpDir + "/**/*.out")
	require.Error(t, walkErr)
}

func TestIsMatched_SlashPattern(t *testing.T) {
	t.Parallel()

	matched := isMatched("base/sub/file.txt", "sub/file.txt", "sub/*.txt")
	assert.True(t, matched)

	matchedFalse := isMatched("base/sub/file.txt", "sub/file.txt", "sub/*.out")
	assert.False(t, matchedFalse)
}

//nolint:paralleltest // t.Chdir cannot be run in parallel
func TestRunMain(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Chdir(filepath.Join(wd, "../.."))

	code := runMain([]string{"-f", "testdata/testdata_set.txt", "-o", os.DevNull})
	assert.Equal(t, 0, code)

	codeErr := runMain([]string{"--invalid-flag"})
	assert.Equal(t, 1, codeErr)
}
