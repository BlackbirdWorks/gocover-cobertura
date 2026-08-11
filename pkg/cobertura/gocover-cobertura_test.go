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

	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const SaveTestResults = false

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

func TestConvert_Synchronous(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("mode: set\ntestdata/func1.go:4.14,5.16 1 1\n")
	var out bytes.Buffer
	err := cobertura.Convert(in, &out)
	require.NoError(t, err)
	assert.NotEmpty(t, out.String())
}

func TestConvert_XMLHeaders(t *testing.T) {
	t.Parallel()
	fname := filepath.Join(t.TempDir(), "stdout")

	temp, err := os.Create(fname)
	require.NoError(t, err)

	err = cobertura.Convert(os.Stdin, temp)
	require.NoError(t, err)
	require.NoError(t, temp.Close())

	outputBytes, err := os.ReadFile(fname)
	require.NoError(t, err)

	outputString := string(outputBytes)
	assert.Contains(t, outputString, xml.Header)
	assert.Contains(t, outputString, cobertura.CoberturaDTDDecl)
}

func TestConvert_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		expectedError string
		closeWriter   bool
	}{
		{
			name:          "parse profiles error",
			input:         "invalid data",
			expectedError: "failed to parse profiles",
			closeWriter:   false,
		},
		{
			name:          "process profiles error",
			input:         "mode: set\nnonexistent.go:1.1,2.2 1 1\n",
			expectedError: "failed to process profiles",
			closeWriter:   false,
		},
		{
			name:          "output error (closed pipe)",
			input:         "mode: set",
			expectedError: "io: read/write on closed pipe",
			closeWriter:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pipe2rd, pipe2wr := io.Pipe()
			if tt.closeWriter {
				require.NoError(t, pipe2wr.Close())
			}
			defer func() { _ = pipe2rd.Close(); _ = pipe2wr.Close() }()

			err := cobertura.Convert(strings.NewReader(tt.input), pipe2wr)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestConvert_WriteErrors(t *testing.T) {
	t.Parallel()

	input := "mode: set\ntestdata/func1.go:4.14,5.16 1 1\n"
	for _, failAfter := range []int{0, 20, 50, 150, 350} {
		fw := &failWriter{failAfter: failAfter}
		bw := bufio.NewWriterSize(fw, 1)
		err := cobertura.Convert(strings.NewReader(input), bw)
		require.Error(t, err)
	}
}

func TestParseProfiles_MultipleProfileErrors(t *testing.T) {
	t.Parallel()

	cov := &cobertura.Coverage{}
	profiles := []*cobertura.Profile{
		{FileName: "does-not-exist-1"},
		{FileName: "does-not-exist-2"},
	}

	err := cov.ParseProfiles(profiles)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't find \"does-not-exist-1\"")
	assert.Contains(t, err.Error(), "can't find \"does-not-exist-2\"")
}

func TestParseProfile_ParseFileError(t *testing.T) {
	t.Parallel()

	tmpfile, err := os.CreateTemp(t.TempDir(), "*.go")
	require.NoError(t, err)
	_, err = tmpfile.WriteString("package invalid syntax %%%")
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	cov := &cobertura.Coverage{}
	err = cov.ParseProfile(&cobertura.Profile{FileName: tmpfile.Name()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse file failed")
}

func TestConvert_Empty(t *testing.T) {
	t.Parallel()
	data := `mode: set`

	pipe2rd, pipe2wr := io.Pipe()
	go func() {
		_ = cobertura.Convert(strings.NewReader(data), pipe2wr)
		_ = pipe2wr.Close()
	}()

	v := cobertura.Coverage{}
	dec := xml.NewDecoder(pipe2rd)
	require.NoError(t, dec.Decode(&v))

	assert.Equal(t, "coverage", v.XMLName.Local)
	assert.NotNil(t, v.Sources)
	assert.Nil(t, v.Packages)
}

func TestParseProfile_Errors(t *testing.T) {
	t.Parallel()

	tmpfile, err := os.CreateTemp(t.TempDir(), "not-readable")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpfile.Name()) })
	require.NoError(t, tmpfile.Chmod(000))

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

func TestConvert_SetMode(t *testing.T) {
	t.Parallel()

	pipe1rd, err := os.Open("testdata/testdata_set.txt")
	require.NoError(t, err, "Can't parse testdata")

	pipe2rd, pipe2wr := io.Pipe()

	var convwr io.Writer = pipe2wr
	if SaveTestResults {
		testwr, err2 := os.Create("testdata/testdata_set.xml")
		require.NoError(t, err2, "Can't open output testdata")
		defer testwr.Close()
		convwr = io.MultiWriter(convwr, testwr)
	}

	go func() {
		_ = cobertura.Convert(pipe1rd, convwr)
		_ = pipe2wr.Close()
	}()

	v := cobertura.Coverage{}
	dec := xml.NewDecoder(pipe2rd)
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
}
