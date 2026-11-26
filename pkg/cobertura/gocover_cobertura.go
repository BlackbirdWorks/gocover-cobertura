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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blackbirdworks/gocover-cobertura/pkg/errs"
)

// CoberturaDTDDecl is the standard DTD declaration for Cobertura XML.
const CoberturaDTDDecl = "<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-04.dtd\">\n"

// Sentinel errors returned during Cobertura conversion and profile parsing.
var (
	ErrProcessProfiles = errors.New("failed to process profiles")
	ErrWriteXMLHeader  = errors.New("failed to write XML header")
	ErrWriteDTD        = errors.New("failed to write DTD declaration")
	ErrEncodeXML       = errors.New("failed to encode XML")
	ErrWriteNewline    = errors.New("failed to write newline")
	ErrFlushBuffer     = errors.New("failed to flush buffer")
	ErrParseProfiles   = errors.New("failed to parse profiles")
	ErrFindFile        = errors.New("find file failed")
	ErrReadFile        = errors.New("read file failed")
	ErrParseFile       = errors.New("parse file failed")
)

// Convert reads Go coverage profiles from the given reader and writes the Cobertura XML format to the writer.
func Convert(reader io.Reader, out io.Writer, opts ...Option) error {
	bufOut := bufio.NewWriter(out)
	if bw, ok := out.(*bufio.Writer); ok {
		bufOut = bw
	}

	covParser := NewParser(opts...)

	coverage, err := covParser.Parse(reader)
	if err != nil {
		return errs.Wrap(err, ErrProcessProfiles)
	}

	_, err = bufOut.WriteString(xml.Header)
	if err != nil {
		return errs.Wrap(err, ErrWriteXMLHeader)
	}

	_, err = bufOut.WriteString(CoberturaDTDDecl)
	if err != nil {
		return errs.Wrap(err, ErrWriteDTD)
	}

	encoder := xml.NewEncoder(bufOut)
	encoder.Indent("", "\t")

	err = encoder.Encode(coverage)
	if err != nil {
		return errs.Wrap(err, ErrEncodeXML)
	}

	err = bufOut.WriteByte('\n')
	if err != nil {
		return errs.Wrap(err, ErrWriteNewline)
	}

	err = bufOut.Flush()
	if err != nil {
		return errs.Wrap(err, ErrFlushBuffer)
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

type cachedImport struct {
	err error
	dir string
}

// NewParser creates a new Parser with the provided options.
func NewParser(opts ...Option) *Parser {
	options := &Options{FS: nil}
	for _, opt := range opts {
		opt(options)
	}

	return &Parser{fsys: options.FS}
}

// Parse processes coverage profile data from an [io.Reader] and generates a Coverage report.
func (p *Parser) Parse(reader io.Reader) (*Coverage, error) {
	profiles, err := ParseProfiles(reader)
	if err != nil {
		return nil, errs.Wrap(err, ErrParseProfiles)
	}

	srcDirs := build.Default.SrcDirs()

	sources := make([]*Source, len(srcDirs))
	for i, dir := range srcDirs {
		sources[i] = &Source{dir}
	}

	cov := &Coverage{
		Sources:   sources,
		Packages:  []*Package{},
		Timestamp: time.Now().UnixMilli(),
	}
	if len(profiles) == 0 {
		cov.LinesValid = 0
		cov.LinesCovered = 0
		cov.LineRate = 0

		return cov, nil
	}

	store := newSafePackageStore()

	var (
		wg          sync.WaitGroup
		importCache sync.Map
	)

	numWorkers := min(runtime.NumCPU(), len(profiles))

	workCh := make(chan *Profile, len(profiles))
	for _, profile := range profiles {
		workCh <- profile
	}

	close(workCh)

	for range numWorkers {
		wg.Go(func() {
			for prof := range workCh {
				classes, pkgPath, profErr := p.parseProfileFile(prof, &importCache)
				if profErr != nil {
					store.AddError(profErr)

					continue
				}

				store.AddClasses(pkgPath, classes)
			}
		})
	}

	wg.Wait()

	storeErr := store.Error()
	if storeErr != nil {
		return nil, storeErr
	}

	cov.Packages = store.Packages()

	slices.SortFunc(cov.Packages, func(a, b *Package) int {
		return cmp.Compare(a.Name, b.Name)
	})

	for _, pkg := range cov.Packages {
		slices.SortFunc(pkg.Classes, func(a, b *Class) int {
			if c := cmp.Compare(a.Filename, b.Filename); c != 0 {
				return c
			}

			return cmp.Compare(a.Name, b.Name)
		})
		pkg.LineRate = pkg.HitRate()
	}

	cov.LinesValid = cov.NumLines()

	cov.LinesCovered = cov.NumLinesWithHits()
	if cov.LinesValid > 0 {
		cov.LineRate = float32(cov.LinesCovered) / float32(cov.LinesValid)
	}

	return cov, nil
}

func (p *Parser) parseProfileFile(profile *Profile, importCache *sync.Map) (map[string]*Class, string, error) {
	fileName := profile.FileName

	absFilePath, err := p.findFile(fileName, importCache)
	if err != nil {
		return nil, "", err
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
		return nil, "", errs.Wrap(err, ErrReadFile)
	}

	fset := token.NewFileSet()

	parsed, err := parser.ParseFile(fset, absFilePath, data, 0)
	if err != nil {
		return nil, "", errs.Wrap(err, ErrParseFile)
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

	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			visitor.visitFunc(fn)
		}
	}

	for _, class := range visitor.classes {
		class.LineRate = class.Lines.HitRate()
	}

	return visitor.classes, pkgPath, nil
}

type fileVisitor struct {
	fset     *token.FileSet
	classes  map[string]*Class
	profile  *Profile
	fileName string
	fileData []byte
}

func (v *fileVisitor) visitFunc(n *ast.FuncDecl) {
	class := v.class(n)
	method := v.method(n)
	method.LineRate = method.Lines.HitRate()
	class.Methods = append(class.Methods, method)
	class.Lines = append(class.Lines, method.Lines...)
}

func (v *fileVisitor) method(n *ast.FuncDecl) *Method {
	method := &Method{Name: n.Name.Name}
	method.Lines = []*Line{}

	start := v.fset.PositionFor(n.Pos(), false)
	end := v.fset.PositionFor(n.End(), false)
	startLine := start.Line
	startCol := start.Column
	endLine := end.Line
	endCol := end.Column
	blocks := v.profile.Blocks
	startIdx := sort.Search(len(blocks), func(i int) bool {
		return blocks[i].StartLine >= startLine
	})
	// The blocks are sorted by start position, so we can stop counting once we pass the function end.
	for _, b := range blocks[startIdx:] {
		if b.StartLine > endLine || (b.StartLine == endLine && b.StartCol >= endCol) {
			// Past the end of the function.
			break
		}

		if b.EndLine == startLine && b.EndCol <= startCol {
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
	start := v.fset.PositionFor(recv.Pos(), false)
	end := v.fset.PositionFor(recv.End(), false)
	name := string(v.fileData[start.Offset:end.Offset])

	return strings.TrimSpace(strings.TrimLeft(name, "*"))
}
