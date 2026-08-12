package cobertura

import (
	"bufio"
	"cmp"
	"encoding/xml"
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blackbirdworks/gocover-cobertura/pkg/errs"
)

// CoberturaDTDDecl is the standard DTD declaration for Cobertura XML.
const CoberturaDTDDecl = "<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-04.dtd\">\n"

// Convert reads Go coverage profiles from the given reader and writes the Cobertura XML format to the writer.
func Convert(in io.Reader, out io.Writer, opts ...Option) error {
	bufOut := bufio.NewWriter(out)
	if bw, ok := out.(*bufio.Writer); ok {
		bufOut = bw
	}

	p := NewParser(opts...)
	coverage, err := p.Parse(in)
	if err != nil {
		return errs.Wrap(err, "failed to process profiles")
	}

	if _, err = out.Write([]byte(xml.Header)); err != nil {
		return errs.Wrap(err, "failed to write XML header")
	}

	if _, err = out.Write([]byte(CoberturaDTDDecl)); err != nil {
		return errs.Wrap(err, "failed to write DTD declaration")
	}

	encoder := xml.NewEncoder(bufOut)
	encoder.Indent("", "\t")
	if err = encoder.Encode(coverage); err != nil {
		return errs.Wrap(err, "failed to encode XML")
	}

	if _, err = out.Write([]byte{'\n'}); err != nil {
		return errs.Wrap(err, "failed to write newline")
	}

	if err = bufOut.Flush(); err != nil {
		return errs.Wrap(err, "failed to flush buffer")
	}

	return nil
}

// safePackageStore manages thread-safe accumulation of parsed packages and classes.
type safePackageStore struct {
	packages map[string]*Package
	errs     []error
	mu       sync.RWMutex
}

func newSafePackageStore() *safePackageStore {
	return &safePackageStore{
		packages: make(map[string]*Package),
	}
}

func (s *safePackageStore) AddClasses(pkgPath string, classes map[string]*Class) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg, exists := s.packages[pkgPath]
	if !exists {
		pkg = &Package{Name: pkgPath, Classes: []*Class{}}
		s.packages[pkgPath] = pkg
	}

	for _, class := range classes {
		pkg.Classes = append(pkg.Classes, class)
	}
	pkg.LineRate = pkg.HitRate()
}

func (s *safePackageStore) AddError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.errs = append(s.errs, err)
}

func (s *safePackageStore) Packages() []*Package {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pkgs := make([]*Package, 0, len(s.packages))
	for _, pkg := range s.packages {
		pkgs = append(pkgs, pkg)
	}

	return pkgs
}

func (s *safePackageStore) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.errs) == 0 {
		return nil
	}

	return errors.Join(s.errs...)
}

// Parser manages the processing of Go coverage profiles into a Coverage representation.
type Parser struct {
	fsys fs.FS
}

// NewParser creates a new Parser with the provided options.
func NewParser(opts ...Option) *Parser {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}

	return &Parser{fsys: o.FS}
}

// Parse processes coverage profile data from an io.Reader and generates a Coverage report.
func (p *Parser) Parse(in io.Reader) (*Coverage, error) {
	profiles, err := ParseProfiles(bufio.NewReader(in))
	if err != nil {
		return nil, errs.Wrap(err, "failed to parse profiles")
	}

	srcDirs := build.Default.SrcDirs()
	sources := make([]*Source, len(srcDirs))
	for i, dir := range srcDirs {
		sources[i] = &Source{dir}
	}

	cov := &Coverage{
		Sources:   sources,
		Packages:  []*Package{},
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}
	if len(profiles) == 0 {
		cov.LinesValid = 0
		cov.LinesCovered = 0
		cov.LineRate = 0

		return cov, nil
	}

	store := newSafePackageStore()
	var wg sync.WaitGroup

	numWorkers := min(runtime.NumCPU(), len(profiles))

	workCh := make(chan *Profile, len(profiles))
	for _, profile := range profiles {
		workCh <- profile
	}
	close(workCh)

	for range numWorkers {
		wg.Add(1)
		//nolint:modernize // waitgroupgo is experimental/buggy
		go func() {
			defer wg.Done()
			for prof := range workCh {
				classes, pkgPath, profErr := p.parseProfileFile(prof)
				if profErr != nil {
					store.AddError(profErr)

					continue
				}
				store.AddClasses(pkgPath, classes)
			}
		}()
	}
	wg.Wait()

	if storeErr := store.Error(); storeErr != nil {
		return nil, storeErr
	}

	cov.Packages = store.Packages()

	slices.SortFunc(cov.Packages, func(a, b *Package) int {
		return cmp.Compare(a.Name, b.Name)
	})
	for _, pkg := range cov.Packages {
		slices.SortFunc(pkg.Classes, func(a, b *Class) int {
			return cmp.Compare(a.Filename, b.Filename)
		})
	}

	cov.LinesValid = cov.NumLines()
	cov.LinesCovered = cov.NumLinesWithHits()
	cov.LineRate = cov.HitRate()

	return cov, nil
}

func (p *Parser) parseProfileFile(profile *Profile) (map[string]*Class, string, error) {
	fileName := profile.FileName
	absFilePath, err := p.findFile(fileName)
	if err != nil {
		return nil, "", errs.Wrap(err, "find file failed")
	}

	var data []byte
	if p.fsys != nil {
		data, err = fs.ReadFile(p.fsys, strings.TrimPrefix(absFilePath, "/"))
		if err != nil {
			data, err = fs.ReadFile(p.fsys, absFilePath)
		}
	} else {
		data, err = os.ReadFile(absFilePath)
	}

	if err != nil {
		return nil, "", errs.Wrap(err, "read file failed")
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, absFilePath, data, 0)
	if err != nil {
		return nil, "", errs.Wrap(err, "parse file failed")
	}

	pkgPath, _ := filepath.Split(fileName)
	pkgPath = strings.TrimRight(pkgPath, string(os.PathSeparator))

	visitor := &fileVisitor{
		fset:     fset,
		fileName: fileName,
		fileData: data,
		classes:  make(map[string]*Class),
		profile:  profile,
	}
	ast.Walk(visitor, parsed)

	return visitor.classes, pkgPath, nil
}

type fileVisitor struct {
	fset     *token.FileSet
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
