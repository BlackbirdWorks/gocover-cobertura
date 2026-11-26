package cobertura_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
)

var errTestRead = errors.New("read error")

type errReader struct{}

func (*errReader) Read([]byte) (int, error) {
	return 0, errTestRead
}

func TestParseProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		reader        io.Reader
		expectedError error
		name          string
		errContains   string
		expectErr     bool
	}{
		{
			name:        "scanner error",
			reader:      &errReader{},
			expectErr:   true,
			errContains: "read error",
		},
		{
			name:          "invalid line",
			reader:        strings.NewReader("mode: set\ninvalid line format"),
			expectErr:     true,
			expectedError: cobertura.ErrBadFormat,
		},
		{
			name:          "invalid first line",
			reader:        strings.NewReader("not_a_mode_header\n"),
			expectErr:     true,
			expectedError: cobertura.ErrBadMode,
		},
		{
			name: "integer overflow",
			reader: strings.NewReader(
				"mode: set\npkg/file.go:999999999999999999999999999999999999999.1,2.2 1 1\n",
			),
			expectErr:     true,
			expectedError: cobertura.ErrBadFormat,
		},
		{
			name:          "missing mode",
			reader:        strings.NewReader("pkg/file.go:1.1,2.2 1 1\n"),
			expectErr:     true,
			expectedError: cobertura.ErrBadMode,
		},
		{
			name:          "empty mode header",
			reader:        strings.NewReader("mode: \n"),
			expectErr:     true,
			expectedError: cobertura.ErrBadMode,
		},
		{
			name:      "successful parse with same line multiple blocks",
			reader:    strings.NewReader("mode: set\nfoo.go:1.4,2.2 1 1\nfoo.go:1.2,2.2 1 0\n"),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := cobertura.ParseProfiles(tt.reader)
			if tt.expectErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				if tt.expectedError != nil {
					require.ErrorIs(t, err, tt.expectedError)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProfile_Boundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile *cobertura.Profile
		name    string
		src     []byte
	}{
		{
			name: "boundaries regular",
			profile: &cobertura.Profile{
				FileName: "foo.go",
				Mode:     "set",
				Blocks: []cobertura.ProfileBlock{
					{StartLine: 1, StartCol: 4, EndLine: 2, EndCol: 2, NumStmt: 1, Count: 1},
					{StartLine: 1, StartCol: 2, EndLine: 2, EndCol: 2, NumStmt: 1, Count: 0},
					{StartLine: 2, StartCol: 2, EndLine: 3, EndCol: 2, NumStmt: 1, Count: 0},
				},
			},
			src: []byte("package foo\nfunc Foo() {}\n"),
		},
		{
			name: "boundaries sort coverage",
			profile: &cobertura.Profile{
				FileName: "foo.go",
				Mode:     "count",
				Blocks: []cobertura.ProfileBlock{
					{StartLine: 1, StartCol: 2, EndLine: 1, EndCol: 2, NumStmt: 1, Count: 5},
					{StartLine: 1, StartCol: 2, EndLine: 1, EndCol: 2, NumStmt: 1, Count: 2},
				},
			},
			src: []byte("package foo\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			boundaries := tt.profile.Boundaries(tt.src)
			assert.NotEmpty(t, boundaries)
		})
	}
}

func TestParseProfile_BuildImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile   *cobertura.Profile
		name      string
		expectErr bool
	}{
		{
			name: "successful import",
			profile: &cobertura.Profile{
				FileName: "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura/profile.go",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := cobertura.NewParser()

			_, err := p.Parse(strings.NewReader("mode: set\n" + tt.profile.FileName + ":1.1,2.2 1 1\n"))
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseProfile_BuildImportCache(t *testing.T) {
	t.Parallel()

	parser := cobertura.NewParser()
	input := "mode: set\n" +
		"github.com/blackbirdworks/gocover-cobertura/pkg/cobertura/profile.go:1.1,2.2 1 1\n" +
		"github.com/blackbirdworks/gocover-cobertura/pkg/cobertura/options.go:1.1,2.2 1 1\n"
	_, err := parser.Parse(strings.NewReader(input))
	require.NoError(t, err)

	// Re-parse to exercise cache hit deterministically without concurrency race.
	_, err = parser.Parse(strings.NewReader(input))
	require.NoError(t, err)
}

func BenchmarkCoverageLine_UnmarshalText(b *testing.B) {
	line := []byte("github.com/blackbirdworks/gocover-cobertura/pkg/cobertura/profile.go:130.1,135.2 5 1")

	var covLine cobertura.CoverageLine

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = covLine.UnmarshalText(line)
	}
}

func BenchmarkParseProfiles(b *testing.B) {
	var builder strings.Builder
	builder.WriteString("mode: set\n")

	for range 200 {
		builder.WriteString("github.com/blackbirdworks/gocover-cobertura/pkg/cobertura/profile.go:130.1,135.2 5 1\n")
	}

	data := builder.String()
	reader := strings.NewReader("")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		reader.Reset(data)

		_, err := cobertura.ParseProfiles(reader)
		require.NoError(b, err)
	}
}
