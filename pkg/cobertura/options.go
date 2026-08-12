package cobertura

import "io/fs"

// Options holds configuration for the Parser.
type Options struct {
	FS fs.FS
}

// Option configures the Parser.
type Option func(*Options)

// WithFS allows supplying a custom filesystem for looking up Go source files
// during AST parsing. By default, the Parser uses the physical OS filesystem.
func WithFS(fsys fs.FS) Option {
	return func(o *Options) {
		o.FS = fsys
	}
}
