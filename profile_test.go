package cobertura_test

import (
	"errors"
	"strings"
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestRead = errors.New("read error")

type errReader struct{}

func (e *errReader) Read([]byte) (int, error) {
	return 0, errTestRead
}

func TestParseProfiles_ScannerError(t *testing.T) {
	t.Parallel()

	_, err := cobertura.ParseProfiles(&errReader{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

func TestParseProfiles_InvalidLine(t *testing.T) {
	t.Parallel()

	input := "mode: set\ninvalid line format"
	_, err := cobertura.ParseProfiles(strings.NewReader(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, cobertura.ErrBadFormat)
}

func TestParseProfiles_InvalidFirstLine(t *testing.T) {
	t.Parallel()

	input := "not_a_mode_header\n"
	_, err := cobertura.ParseProfiles(strings.NewReader(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, cobertura.ErrBadMode)
}

func TestParseProfiles_IntegerOverflow(t *testing.T) {
	t.Parallel()

	input := "mode: set\npkg/file.go:999999999999999999999999999999999999999.1,2.2 1 1\n"
	_, err := cobertura.ParseProfiles(strings.NewReader(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, cobertura.ErrBadFormat)
}

func TestParseProfiles_MissingMode(t *testing.T) {
	t.Parallel()

	input := "pkg/file.go:1.1,2.2 1 1\n"
	_, err := cobertura.ParseProfiles(strings.NewReader(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, cobertura.ErrBadMode)
}

func TestParseProfiles_EmptyModeHeader(t *testing.T) {
	t.Parallel()

	input := "mode: \n"
	_, err := cobertura.ParseProfiles(strings.NewReader(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, cobertura.ErrBadMode)
}

func TestProfile_Boundaries(t *testing.T) {
	t.Parallel()

	profile := &cobertura.Profile{
		FileName: "foo.go",
		Mode:     "set",
		Blocks: []cobertura.ProfileBlock{
			{StartLine: 1, StartCol: 4, EndLine: 2, EndCol: 2, NumStmt: 1, Count: 1},
			{StartLine: 1, StartCol: 2, EndLine: 2, EndCol: 2, NumStmt: 1, Count: 0},
			{StartLine: 2, StartCol: 2, EndLine: 3, EndCol: 2, NumStmt: 1, Count: 0},
		},
	}

	src := []byte("package foo\nfunc Foo() {}\n")
	boundaries := profile.Boundaries(src)
	assert.NotEmpty(t, boundaries)
}

func TestProfile_Boundaries_SortCoverage(t *testing.T) {
	t.Parallel()

	profile := &cobertura.Profile{
		FileName: "foo.go",
		Mode:     "count",
		Blocks: []cobertura.ProfileBlock{
			{StartLine: 1, StartCol: 2, EndLine: 1, EndCol: 2, NumStmt: 1, Count: 5},
			{StartLine: 1, StartCol: 2, EndLine: 1, EndCol: 2, NumStmt: 1, Count: 2},
		},
	}

	src := []byte("package foo\n")
	boundaries := profile.Boundaries(src)
	assert.NotEmpty(t, boundaries)
}

func TestParseProfile_BuildImport(t *testing.T) {
	t.Parallel()

	cov := &cobertura.Coverage{}
	err := cov.ParseProfile(&cobertura.Profile{
		FileName: "github.com/blackbirdworks/gocover-cobertura/profile.go",
	})
	require.NoError(t, err)
}
