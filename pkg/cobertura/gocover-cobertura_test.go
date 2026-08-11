package cobertura_test

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestWriter = errors.New("writer failed")

type failWriter struct {
	failAfter int
	written   int
}

func (f *failWriter) Write(p []byte) (int, error) {
	if f.written+len(p) > f.failAfter {
		return 0, errTestWriter
	}
	f.written += len(p)

	return len(p), nil
}

func TestConvert_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    func(t *testing.T) io.Reader
		validate func(t *testing.T, output []byte)
		name     string
	}{
		{
			name: "synchronous",
			input: func(t *testing.T) io.Reader {
				t.Helper()

				return strings.NewReader("mode: set\ntestdata/func1.go:4.14,5.16 1 1\n")
			},
			validate: func(t *testing.T, output []byte) {
				t.Helper()
				assert.NotEmpty(t, output)
			},
		},
		{
			name: "xml headers",
			input: func(t *testing.T) io.Reader {
				t.Helper()

				return os.Stdin
			},
			validate: func(t *testing.T, output []byte) {
				t.Helper()
				outputString := string(output)
				assert.Contains(t, outputString, xml.Header)
				assert.Contains(t, outputString, cobertura.CoberturaDTDDecl)
			},
		},
		{
			name: "empty",
			input: func(t *testing.T) io.Reader {
				t.Helper()

				return strings.NewReader("mode: set")
			},
			validate: func(t *testing.T, output []byte) {
				t.Helper()
				v := cobertura.Coverage{}
				dec := xml.NewDecoder(bytes.NewReader(output))
				require.NoError(t, dec.Decode(&v))

				assert.Equal(t, "coverage", v.XMLName.Local)
				assert.NotNil(t, v.Sources)
				assert.Nil(t, v.Packages)
			},
		},
		{
			name: "set mode",
			input: func(t *testing.T) io.Reader {
				t.Helper()
				pipe1rd, err := os.Open("testdata/testdata_set.txt")
				require.NoError(t, err, "Can't parse testdata")

				return pipe1rd
			},
			validate: func(t *testing.T, output []byte) {
				t.Helper()
				v := cobertura.Coverage{}
				dec := xml.NewDecoder(bytes.NewReader(output))
				require.NoError(t, dec.Decode(&v))

				assert.Equal(t, "coverage", v.XMLName.Local)
				require.NotNil(t, v.Sources)
				require.Len(t, v.Packages, 1)

				p := v.Packages[0]
				assert.Equal(t, "./testdata", strings.TrimRight(p.Name, "/"))
				assert.Greater(t, float64(p.HitRate()), 0.0)

				require.Len(t, p.Classes, 2)

				c := p.Classes[0]
				assert.Equal(t, "-", c.Name)
				assert.Equal(t, "./testdata/func1.go", c.Filename)
				assert.InEpsilon(t, 0.25, c.HitRate(), 0.01)

				require.Len(t, c.Methods, 1)
				require.Len(t, c.Lines, 4)

				m := c.Methods[0]
				assert.Equal(t, "Func1", m.Name)
				assert.InEpsilon(t, 0.25, m.HitRate(), 0.01)
				require.Len(t, m.Lines, 4)

				expectedLines := []struct {
					number int
					hits   int64
				}{
					{4, int64(1)},
					{5, int64(0)},
					{6, int64(0)},
					{7, int64(0)},
				}

				for i, expected := range expectedLines {
					l := m.Lines[i]
					assert.Equal(t, expected.number, l.Number)
					assert.Equal(t, expected.hits, l.Hits)

					cl := c.Lines[i]
					assert.Equal(t, expected.number, cl.Number)
					assert.Equal(t, expected.hits, cl.Hits)
				}

				c = p.Classes[1]
				assert.Equal(t, "Type1", c.Name)
				assert.Equal(t, "./testdata/func2.go", c.Filename)
				assert.Len(t, c.Methods, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := tt.input(t)
			if closer, ok := in.(io.Closer); ok && in != os.Stdin {
				defer closer.Close()
			}

			var out bytes.Buffer
			err := cobertura.Convert(in, &out)
			require.NoError(t, err)
			tt.validate(t, out.Bytes())
		})
	}
}

func TestConvert_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setupWriter   func(t *testing.T) (io.Writer, func())
		name          string
		input         string
		expectedError string
	}{
		{
			name:          "parse profiles error",
			input:         "invalid data",
			expectedError: "failed to parse profiles",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				var b bytes.Buffer

				return &b, func() {}
			},
		},
		{
			name:          "process profiles error",
			input:         "mode: set\nnonexistent.go:1.1,2.2 1 1\n",
			expectedError: "failed to process profiles",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				var b bytes.Buffer

				return &b, func() {}
			},
		},
		{
			name:          "output error closed pipe",
			input:         "mode: set",
			expectedError: "io: read/write on closed pipe",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				pipe2rd, pipe2wr := io.Pipe()
				require.NoError(t, pipe2wr.Close())

				return pipe2wr, func() { _ = pipe2rd.Close() }
			},
		},
		{
			name:          "write error after 0 bytes",
			input:         "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n",
			expectedError: errTestWriter.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				fw := &failWriter{failAfter: 0}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error after 20 bytes",
			input:         "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n",
			expectedError: errTestWriter.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				fw := &failWriter{failAfter: 20}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error after 50 bytes",
			input:         "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n",
			expectedError: errTestWriter.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				fw := &failWriter{failAfter: 50}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error on newline",
			input:         "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n",
			expectedError: "failed to write newline",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				fw := &failWriter{failAfter: 0}

				return bufio.NewWriterSize(fw, 985), func() {}
			},
		},
		{
			name:          "flush error",
			input:         "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n",
			expectedError: "failed to flush buffer",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()
				fw := &failWriter{failAfter: 985}

				return bufio.NewWriterSize(fw, 4096), func() {}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, cleanup := tt.setupWriter(t)
			defer cleanup()

			err := cobertura.Convert(strings.NewReader(tt.input), w)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestParseProfiles_MultipleProfileErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		errContains1 string
		errContains2 string
		profiles     []*cobertura.Profile
	}{
		{
			name: "multiple not found",
			profiles: []*cobertura.Profile{
				{FileName: "does-not-exist-1"},
				{FileName: "does-not-exist-2"},
			},
			errContains1: "can't find \"does-not-exist-1\"",
			errContains2: "can't find \"does-not-exist-2\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cov := &cobertura.Coverage{}
			err := cov.ParseProfiles(tt.profiles)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains1)
			assert.Contains(t, err.Error(), tt.errContains2)
		})
	}
}

func TestParseProfile_Errors(t *testing.T) {
	t.Parallel()

	tmpfile, err := os.CreateTemp(t.TempDir(), "not-readable")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpfile.Name()) })
	require.NoError(t, tmpfile.Chmod(000))

	tmpfileSyntax, err := os.CreateTemp(t.TempDir(), "*.go")
	require.NoError(t, err)
	_, err = tmpfileSyntax.WriteString("package invalid syntax %%%")
	require.NoError(t, err)
	require.NoError(t, tmpfileSyntax.Close())

	tests := []struct {
		name          string
		fileName      string
		expectedError string
	}{
		{
			name:          "does not exist",
			fileName:      "does-not-exist",
			expectedError: "can't find \"does-not-exist\"",
		},
		{
			name:          "not readable",
			fileName:      os.DevNull,
			expectedError: "expected 'package', found 'EOF'",
		},
		{
			name:          "permission denied",
			fileName:      tmpfile.Name(),
			expectedError: "permission denied",
		},
		{
			name:          "parse file error",
			fileName:      tmpfileSyntax.Name(),
			expectedError: "parse file failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := cobertura.Coverage{}
			profile := cobertura.Profile{FileName: tt.fileName}
			parseErr := v.ParseProfile(&profile)

			require.Error(t, parseErr)
			assert.Contains(t, parseErr.Error(), tt.expectedError)
		})
	}
}
