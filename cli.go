package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
)

// ErrNoMatches is returned when a glob pattern does not match any files.
var ErrNoMatches = errors.New("no files matched pattern")

const globSplitParts = 2

// CLI represents the command line configuration for gocover-cobertura.
type CLI struct {
	File    string `default:"-" help:"Input coverage file path (default '-' for stdin)"        short:"f"`
	Pattern string `default:""  help:"Glob pattern for matching files (e.g. '**/*.out')"       short:"p"`
	Output  string `default:"-" help:"Output Cobertura XML file path (default '-' for stdout)" short:"o"`
}

// Run executes the CLI logic using guard clauses without else statements.
func (c *CLI) Run() error {
	in, err := c.openInput()
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := c.openOutput()
	if err != nil {
		return err
	}
	defer out.Close()

	if err = cobertura.Convert(in, out); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	return nil
}

func (c *CLI) openInput() (io.ReadCloser, error) {
	if c.Pattern != "" {
		return c.openPatternInput()
	}

	if c.File != "" && c.File != "-" {
		f, err := os.Open(c.File)
		if err != nil {
			return nil, fmt.Errorf("failed to open input file %q: %w", c.File, err)
		}

		return f, nil
	}

	return io.NopCloser(os.Stdin), nil
}

func (c *CLI) openPatternInput() (io.ReadCloser, error) {
	matches, err := findMatchingFiles(c.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to process pattern %q: %w", c.Pattern, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w %q", ErrNoMatches, c.Pattern)
	}

	var closers []io.Closer
	var readers []io.Reader
	for _, match := range matches {
		fileHandle, openErr := os.Open(match)
		if openErr != nil {
			for _, c := range closers {
				_ = c.Close()
			}

			return nil, fmt.Errorf("failed to open matched file %q: %w", match, openErr)
		}
		closers = append(closers, fileHandle)
		readers = append(readers, fileHandle)
	}

	return &multiReadCloser{
		Reader:  io.MultiReader(readers...),
		closers: closers,
	}, nil
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (m *multiReadCloser) Close() error {
	var errs []error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (c *CLI) openOutput() (io.WriteCloser, error) {
	if c.Output == "" || c.Output == "-" {
		return nopWriteCloser{Writer: os.Stdout}, nil
	}

	f, err := os.Create(c.Output)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %q: %w", c.Output, err)
	}

	return f, nil
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

// findMatchingFiles finds files matching a pattern, supporting recursive ** matching.
func findMatchingFiles(pattern string) ([]string, error) {
	if !strings.Contains(pattern, "**") {
		return filepath.Glob(pattern)
	}

	parts := strings.SplitN(pattern, "**", globSplitParts)
	baseDir := parts[0]
	if baseDir == "" {
		baseDir = "."
	} else {
		baseDir = filepath.Clean(baseDir)
	}

	subPattern := strings.TrimPrefix(parts[1], string(os.PathSeparator))
	subPattern = strings.TrimPrefix(subPattern, "/")
	if subPattern == "" {
		subPattern = "*"
	}

	return walkMatchingFiles(baseDir, subPattern)
}

func walkMatchingFiles(baseDir, subPattern string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}

			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath := path
		if baseDir != "." {
			relPath = strings.TrimPrefix(path, baseDir+string(os.PathSeparator))
		}

		if isMatched(path, relPath, subPattern) {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

func isMatched(path, relPath, subPattern string) bool {
	matched, err := filepath.Match(subPattern, filepath.Base(path))
	if err == nil && matched {
		return true
	}

	if strings.Contains(subPattern, "/") {
		relMatched, relErr := filepath.Match(subPattern, relPath)

		return relErr == nil && relMatched
	}

	return false
}

// RunCLI initializes Kong with arguments and executes the CLI application.
func RunCLI(args []string) error {
	var cli CLI
	parser, err := kong.New(&cli,
		kong.Name("gocover-cobertura"),
		kong.Description("Convert Go cover profile to Cobertura XML format"),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		return err
	}

	_, err = parser.Parse(args)
	if err != nil {
		return err
	}

	return cli.Run()
}
