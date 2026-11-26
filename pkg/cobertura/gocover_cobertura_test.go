package cobertura_test

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blackbirdworks/gocover-cobertura/internal/testfixtures"
	"github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
)

const testSimpleProfile = "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n"

var errTestWriter = errors.New("writer failed")

type failWriter struct {
	failAfter int
	written   int
}

func (f *failWriter) Write(data []byte) (int, error) {
	if f.written+len(data) > f.failAfter {
		return 0, errTestWriter
	}

	f.written += len(data)

	return len(data), nil
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

				return strings.NewReader(testSimpleProfile)
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

				cov := cobertura.Coverage{}
				dec := xml.NewDecoder(bytes.NewReader(output))
				require.NoError(t, dec.Decode(&cov))

				assert.Equal(t, "coverage", cov.XMLName.Local)
				assert.NotNil(t, cov.Sources)
				assert.Nil(t, cov.Packages)
			},
		},
		{
			name: "set mode",
			input: func(t *testing.T) io.Reader {
				t.Helper()
				tmpDir := testfixtures.WriteToTempDir(t)
				pipe1rd, err := os.Open(filepath.Join(tmpDir, "testdata", "testdata_set.txt"))
				require.NoError(t, err, "Can't parse testdata")

				return pipe1rd
			},
			validate: func(t *testing.T, output []byte) {
				t.Helper()

				cov := cobertura.Coverage{}
				dec := xml.NewDecoder(bytes.NewReader(output))
				require.NoError(t, dec.Decode(&cov))

				assert.Equal(t, "coverage", cov.XMLName.Local)
				require.NotNil(t, cov.Sources)
				require.Len(t, cov.Packages, 1)

				pkg := cov.Packages[0]
				assert.Equal(t, "./testdata", strings.TrimRight(pkg.Name, "/"))
				assert.Greater(t, float64(pkg.HitRate()), 0.0)

				require.Len(t, pkg.Classes, 2)

				cls := pkg.Classes[0]
				assert.Equal(t, "-", cls.Name)
				assert.True(t, strings.HasSuffix(cls.Filename, "testdata/func1.go"))
				assert.InEpsilon(t, 0.25, cls.HitRate(), 0.01)

				require.Len(t, cls.Methods, 1)
				require.Len(t, cls.Lines, 4)

				method := cls.Methods[0]
				assert.Equal(t, "Func1", method.Name)
				assert.InEpsilon(t, 0.25, method.HitRate(), 0.01)
				require.Len(t, method.Lines, 4)

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
					l := method.Lines[i]
					assert.Equal(t, expected.number, l.Number)
					assert.Equal(t, expected.hits, l.Hits)

					cl := cls.Lines[i]
					assert.Equal(t, expected.number, cl.Number)
					assert.Equal(t, expected.hits, cl.Hits)
				}

				cls = pkg.Classes[1]
				assert.Equal(t, "Type1", cls.Name)
				assert.True(t, strings.HasSuffix(cls.Filename, "testdata/func2.go"))
				assert.Len(t, cls.Methods, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := tt.input(t)
			if closer, ok := reader.(io.Closer); ok && reader != os.Stdin {
				defer closer.Close()
			}

			var out bytes.Buffer

			err := cobertura.Convert(reader, &out, cobertura.WithFS(testfixtures.FS))
			require.NoError(t, err)
			tt.validate(t, out.Bytes())
		})
	}
}

func TestConvert_UnbufferedOutputFormatting(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	err := cobertura.Convert(
		strings.NewReader(testSimpleProfile),
		&out,
		cobertura.WithFS(testfixtures.FS),
	)
	require.NoError(t, err)

	output := out.String()
	assert.True(t, strings.HasPrefix(output, xml.Header+cobertura.CoberturaDTDDecl+"<coverage "))
	assert.True(t, strings.HasSuffix(output, "</coverage>\n"))
	assert.NotContains(t, output, cobertura.CoberturaDTDDecl+"\n<coverage")
}

func TestParse_SameLineFunctionsAndDeterministicClassSort(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "sameline.go")
	src := "package sample\n" +
		"type S struct{}\n" +
		"func A() int { return 1 }; func (S) B() int { return 2 }\n"
	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0o600))

	profile := "mode: set\n" +
		srcFile + ":3.14,3.26 1 1\n" +
		srcFile + ":3.45,3.57 1 0\n"

	p := cobertura.NewParser()
	cov, err := p.Parse(strings.NewReader(profile))
	require.NoError(t, err)
	require.Len(t, cov.Packages, 1)
	require.Len(t, cov.Packages[0].Classes, 2)
	assert.Equal(t, "-", cov.Packages[0].Classes[0].Name)
	assert.Equal(t, "S", cov.Packages[0].Classes[1].Name)
	assert.InEpsilon(t, float32(0.5), cov.LineRate, 0.01)
}

func TestConvert_Error(t *testing.T) {
	t.Parallel()

	// Dynamically compute the size of the XML output without newline
	// to properly test write boundaries in different environments.
	var sizeBuf bytes.Buffer

	sizeErr := cobertura.Convert(strings.NewReader(testSimpleProfile), &sizeBuf, cobertura.WithFS(testfixtures.FS))
	require.NoError(t, sizeErr)

	xmlSize := sizeBuf.Len() - 1 // subtract the newline

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
			input:         testSimpleProfile,
			expectedError: errTestWriter.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: 0}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error after 20 bytes", // fails during xml header
			input:         testSimpleProfile,
			expectedError: cobertura.ErrWriteXMLHeader.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: 20}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error after 50 bytes", // fails during DTD
			input:         testSimpleProfile,
			expectedError: cobertura.ErrWriteDTD.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: 50}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error after 130 bytes", // fails during encoder.Encode
			input:         testSimpleProfile,
			expectedError: cobertura.ErrEncodeXML.Error(),
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: 130}

				return bufio.NewWriterSize(fw, 1), func() {}
			},
		},
		{
			name:          "write error on newline",
			input:         testSimpleProfile,
			expectedError: "failed to write newline",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: 0}

				return bufio.NewWriterSize(fw, xmlSize), func() {}
			},
		},
		{
			name:          "flush error",
			input:         testSimpleProfile,
			expectedError: "failed to flush buffer",
			setupWriter: func(t *testing.T) (io.Writer, func()) {
				t.Helper()

				fw := &failWriter{failAfter: xmlSize}

				return bufio.NewWriterSize(fw, 4096), func() {}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, cleanup := tt.setupWriter(t)
			defer cleanup()

			err := cobertura.Convert(strings.NewReader(tt.input), w, cobertura.WithFS(testfixtures.FS))
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

			parser := cobertura.NewParser()

			var inputData strings.Builder
			inputData.WriteString("mode: set\n")

			for _, prof := range tt.profiles {
				inputData.WriteString(prof.FileName + ":1.1,2.2 1 1\n")
			}

			_, err := parser.Parse(strings.NewReader(inputData.String()))

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
	require.NoError(t, tmpfile.Chmod(0o000))

	tmpfileSyntax, err := os.CreateTemp(t.TempDir(), "*.go")
	require.NoError(t, err)
	_, err = tmpfileSyntax.WriteString("package invalid syntax %%%")
	require.NoError(t, err)
	require.NoError(t, tmpfileSyntax.Close())

	tests := []struct {
		name          string
		fileName      string
		expectedError string
		useFS         bool
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
		{
			name:          "fsys read file error",
			fileName:      "testdata",
			expectedError: "read file failed",
			useFS:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []cobertura.Option
			if tt.useFS {
				opts = append(opts, cobertura.WithFS(testfixtures.FS))
			}

			p := cobertura.NewParser(opts...)

			_, parseErr := p.Parse(strings.NewReader("mode: set\n" + tt.fileName + ":1.1,2.2 1 1\n"))

			require.Error(t, parseErr)
			assert.Contains(t, parseErr.Error(), tt.expectedError)
		})
	}
}
