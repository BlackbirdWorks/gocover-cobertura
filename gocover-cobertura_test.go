package cobertura_test

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura"
)

const SaveTestResults = false

// removed dirInfo (unused)

func TestMain(t *testing.T) {
	t.Parallel()
	fname := filepath.Join(t.TempDir(), "stdout")

	// this is test code writing a temp file
	temp, _ := os.Create(fname)
	cobertura.Convert(os.Stdin, temp)
	_ = temp.Close()

	outputBytes, err := os.ReadFile(fname)
	if err != nil {
		t.Fail()
	}
	outputString := string(outputBytes)
	if !strings.Contains(outputString, xml.Header) {
		t.Fail()
	}
	if !strings.Contains(outputString, cobertura.CoberturaDTDDeclForTest) {
		t.Fail()
	}
}

func TestConvertParseProfilesError(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil || r != "Can't parse profiles" {
			t.Errorf("The code did not panic as expected; r = %+v", r)
		}
	}()

	pipe2rd, pipe2wr := io.Pipe()
	defer func() { _ = pipe2rd.Close(); _ = pipe2wr.Close() }()
	cobertura.Convert(strings.NewReader("invalid data"), pipe2wr)
}

func TestConvertOutputError(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil || r.(error).Error() != "io: read/write on closed pipe" {
			t.Errorf("The code did not panic as expected; r = %+v", r)
		}
	}()

	pipe2rd, pipe2wr := io.Pipe()
	_ = pipe2wr.Close()
	defer func() { _ = pipe2rd.Close() }()
	cobertura.Convert(strings.NewReader("mode: set"), pipe2wr)
}

func TestConvertEmpty(t *testing.T) {
	t.Parallel()
	data := `mode: set`

	pipe2rd, pipe2wr := io.Pipe()
	go cobertura.Convert(strings.NewReader(data), pipe2wr)

	v := cobertura.Coverage{}
	dec := xml.NewDecoder(pipe2rd)
	_ = dec.Decode(&v)

	if v.XMLName.Local != "coverage" {
		t.Error()
	}
	if v.Sources == nil {
		t.Fatal()
	}
	if v.Packages != nil {
		t.Fatal()
	}
}

func TestParseProfileDoesntExist(t *testing.T) {
	t.Parallel()
	v := cobertura.Coverage{}
	profile := cobertura.Profile{FileName: "does-not-exist"}
	err := v.ParseProfileForTest(&profile)
	if err == nil || !strings.Contains(err.Error(), `can't find "does-not-exist"`) {
		t.Fatalf("Expected \"can't find\" error; got: %+v", err)
	}
}

func TestParseProfileNotReadable(t *testing.T) {
	t.Parallel()
	v := cobertura.Coverage{}
	profile := cobertura.Profile{FileName: os.DevNull}
	err := v.ParseProfileForTest(&profile)
	if err == nil || !strings.Contains(err.Error(), `expected 'package', found 'EOF'`) {
		t.Fatalf("Expected \"expected 'package', found 'EOF'\" error; got: %+v", err)
	}
}

func TestParseProfilePermissionDenied(t *testing.T) {
	t.Parallel()
	tmpfile, _ := os.CreateTemp(t.TempDir(), "not-readable")
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	_ = tmpfile.Chmod(000)
	v := cobertura.Coverage{}
	profile := cobertura.Profile{FileName: tmpfile.Name()}
	err := v.ParseProfileForTest(&profile)
	if err == nil || !strings.Contains(err.Error(), `permission denied`) {
		t.Fatalf("Expected \"permission denied\" error; got: %+v", err)
	}
}

//nolint:gocyclo // test function
func TestConvertSetMode(t *testing.T) {
	t.Parallel()
	pipe1rd, err := os.Open("testdata/testdata_set.txt")
	if err != nil {
		t.Fatal("Can't parse testdata.")
	}

	pipe2rd, pipe2wr := io.Pipe()

	var convwr io.Writer = pipe2wr
	if SaveTestResults {
		testwr, err2 := os.Create("testdata/testdata_set.xml")
		if err2 != nil {
			t.Fatal("Can't open output testdata.", err2)
		}
		defer func() { _ = testwr.Close() }()
		convwr = io.MultiWriter(convwr, testwr)
	}

	go cobertura.Convert(pipe1rd, convwr)

	v := cobertura.Coverage{}
	dec := xml.NewDecoder(pipe2rd)
	_ = dec.Decode(&v)

	if v.XMLName.Local != "coverage" {
		t.Error()
	}

	if v.Sources == nil {
		t.Fatal()
	}

	if v.Packages == nil || len(v.Packages) != 1 {
		t.Fatal()
	}

	p := v.Packages[0]
	if strings.TrimRight(p.Name, "/") != "./testdata" {
		t.Fatal(p.Name)
	}
	if p.Classes == nil || len(p.Classes) != 2 {
		t.Fatal()
	}

	c := p.Classes[0]
	if c.Name != "-" {
		t.Error()
	}
	if c.Filename != "./testdata/func1.go" {
		t.Errorf("Expected %s but %s", "./testdata/func1.go", c.Filename)
	}
	if c.Methods == nil || len(c.Methods) != 1 {
		t.Fatal()
	}
	if c.Lines == nil || len(c.Lines) != 4 {
		t.Errorf("Expected 4 lines but got %d", len(c.Lines))
	}

	m := c.Methods[0]
	if m.Name != "Func1" {
		t.Error()
	}
	if c.Lines == nil || len(c.Lines) != 4 {
		t.Errorf("Expected 4 lines but got %d", len(c.Lines))
	}

	var l *cobertura.Line
	if l = m.Lines[0]; l.Number != 4 || l.Hits != 1 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = m.Lines[1]; l.Number != 5 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = m.Lines[2]; l.Number != 6 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = m.Lines[3]; l.Number != 7 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}

	if l = c.Lines[0]; l.Number != 4 || l.Hits != 1 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = c.Lines[1]; l.Number != 5 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = c.Lines[2]; l.Number != 6 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}
	if l = c.Lines[3]; l.Number != 7 || l.Hits != 0 {
		t.Errorf("unmatched line: Number:%d, Hits:%d", l.Number, l.Hits)
	}

	c = p.Classes[1]
	if c.Name != "Type1" {
		t.Error()
	}
	if c.Filename != "./testdata/func2.go" {
		t.Errorf("Expected %s but %s", "./testdata/func2.go", c.Filename)
	}
	if c.Methods == nil || len(c.Methods) != 3 {
		t.Fatal()
	}
}
