package cobertura

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CoberturaDTDDecl is the standard DTD declaration for Cobertura XML.
const CoberturaDTDDecl = "<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-04.dtd\">\n"

// Convert reads Go coverage profiles from the given reader and writes the Cobertura XML format to the writer.
func Convert(in io.Reader, out io.Writer) error {
	bufIn := bufio.NewReader(in)
	bufOut := bufio.NewWriter(out)

	profiles, err := ParseProfiles(bufIn)
	if err != nil {
		return fmt.Errorf("failed to parse profiles: %w", err)
	}

	srcDirs := build.Default.SrcDirs()
	sources := make([]*Source, len(srcDirs))
	for i, dir := range srcDirs {
		sources[i] = &Source{dir}
	}

	coverage := Coverage{
		Sources:   sources,
		Packages:  nil,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	if err = coverage.ParseProfiles(profiles); err != nil {
		return fmt.Errorf("failed to process profiles: %w", err)
	}

	if _, err = fmt.Fprintf(bufOut, xml.Header); err != nil {
		return fmt.Errorf("failed to write XML header: %w", err)
	}

	if _, err = fmt.Fprint(bufOut, CoberturaDTDDecl); err != nil {
		return fmt.Errorf("failed to write DTD declaration: %w", err)
	}

	encoder := xml.NewEncoder(bufOut)
	encoder.Indent("", "\t")
	if err = encoder.Encode(coverage); err != nil {
		return fmt.Errorf("failed to encode XML: %w", err)
	}

	if _, err = fmt.Fprintln(bufOut); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	if err = bufOut.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	return nil
}

// ParseProfiles processes a slice of coverage profiles and populates the Coverage metrics.
func (cov *Coverage) ParseProfiles(profiles []*Profile) error {
	cov.Packages = []*Package{}
	var errs []error

	for _, profile := range profiles {
		if err := cov.ParseProfile(profile); err != nil {
			errs = append(errs, err)
		}
	}

	cov.LinesValid = cov.NumLines()
	cov.LinesCovered = cov.NumLinesWithHits()
	cov.LineRate = cov.HitRate()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ParseProfile processes a single Profile and updates the Coverage model.
func (cov *Coverage) ParseProfile(profile *Profile) error {
	fileName := profile.FileName
	absFilePath, err := findFile(fileName)
	if err != nil {
		return fmt.Errorf("find file failed: %w", err)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, absFilePath, nil, 0)
	if err != nil {
		return fmt.Errorf("parse file failed: %w", err)
	}

	data, err := os.ReadFile(absFilePath)
	if err != nil {
		return fmt.Errorf("read file failed: %w", err)
	}

	pkgPath, _ := filepath.Split(fileName)
	pkgPath = strings.TrimRight(pkgPath, string(os.PathSeparator))

	var pkg *Package
	for _, p := range cov.Packages {
		if p.Name == pkgPath {
			pkg = p
		}
	}
	if pkg == nil {
		pkg = &Package{Name: pkgPath, Classes: []*Class{}}
		cov.Packages = append(cov.Packages, pkg)
	}
	visitor := &fileVisitor{
		fset:     fset,
		fileName: fileName,
		fileData: data,
		classes:  make(map[string]*Class),
		pkg:      pkg,
		profile:  profile,
	}
	ast.Walk(visitor, parsed)
	pkg.LineRate = pkg.HitRate()

	return nil
}

type fileVisitor struct {
	fset     *token.FileSet
	pkg      *Package
	classes  map[string]*Class
	profile  *Profile
	fileName string
	fileData []byte
}

func (v *fileVisitor) Visit(node ast.Node) ast.Visitor {
	if n, ok := node.(*ast.FuncDecl); ok {
		class := v.class(n)
		method := v.method(n)
		method.LineRate = method.Lines.HitRate()
		class.Methods = append(class.Methods, method)
		class.Lines = append(class.Lines, method.Lines...)
		class.LineRate = class.Lines.HitRate()
	}

	return v
}

func (v *fileVisitor) method(n *ast.FuncDecl) *Method {
	method := &Method{Name: n.Name.Name}
	method.Lines = []*Line{}

	start := v.fset.Position(n.Pos())
	end := v.fset.Position(n.End())
	startLine := start.Line
	startCol := start.Column
	endLine := end.Line
	endCol := end.Column
	// The blocks are sorted, so we can stop counting as soon as we reach the end of the relevant block.
	for _, b := range v.profile.Blocks {
		if b.StartLine > endLine || (b.StartLine == endLine && b.StartCol >= endCol) {
			// Past the end of the function.
			break
		}
		if b.EndLine < startLine || (b.EndLine == startLine && b.EndCol <= startCol) {
			// Before the beginning of the function
			continue
		}
		for i := b.StartLine; i <= b.EndLine; i++ {
			method.Lines.AddOrUpdateLine(i, int64(b.Count))
		}
	}

	return method
}

func (v *fileVisitor) class(n *ast.FuncDecl) *Class {
	className := v.recvName(n)
	class := v.classes[className]
	if class == nil {
		class = &Class{Name: className, Filename: v.fileName, Methods: []*Method{}, Lines: []*Line{}}
		v.classes[className] = class
		v.pkg.Classes = append(v.pkg.Classes, class)
	}

	return class
}

func (v *fileVisitor) recvName(n *ast.FuncDecl) string {
	if n.Recv == nil {
		return "-"
	}
	recv := n.Recv.List[0].Type
	start := v.fset.Position(recv.Pos())
	end := v.fset.Position(recv.End())
	name := string(v.fileData[start.Offset:end.Offset])

	return strings.TrimSpace(strings.TrimLeft(name, "*"))
}
