package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestClose = errors.New("close error")

type failCloser struct{}

func (failCloser) Close() error {
	return errTestClose
}

func TestMultiReadCloser_Close(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		errContains string
		closers     []io.Closer
		expectErr   bool
	}{
		{
			name:      "no errors",
			closers:   []io.Closer{},
			expectErr: false,
		},
		{
			name:        "close error",
			closers:     []io.Closer{failCloser{}},
			expectErr:   true,
			errContains: "close error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mc := &multiReadCloser{
				Reader:  os.Stdin,
				closers: tt.closers,
			}

			err := mc.Close()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCLI_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup       func(t *testing.T) CLI
		validate    func(t *testing.T, cli CLI)
		name        string
		errContains string
		expectErr   bool
	}{
		{
			name: "run with valid file",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()

				return CLI{
					File:   "testdata/testdata_set.txt",
					Output: filepath.Join(tmpDir, "output.xml"),
				}
			},
			validate: func(t *testing.T, cli CLI) {
				t.Helper()
				assert.FileExists(t, cli.Output)
			},
		},
		{
			name: "run with stdin and stdout",
			setup: func(t *testing.T) CLI {
				t.Helper()

				return CLI{File: "-", Output: "-"}
			},
		},
		{
			name: "run with pattern glob",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()

				return CLI{
					Pattern: "testdata/**/*.txt",
					Output:  filepath.Join(tmpDir, "output.xml"),
				}
			},
			validate: func(t *testing.T, cli CLI) {
				t.Helper()
				assert.FileExists(t, cli.Output)
			},
		},
		{
			name: "run with pattern double star only",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()

				return CLI{
					Pattern: "**/testdata_set.txt",
					Output:  filepath.Join(tmpDir, "output.xml"),
				}
			},
			validate: func(t *testing.T, cli CLI) {
				t.Helper()
				assert.FileExists(t, cli.Output)
			},
		},
		{
			name: "run with pattern double star empty",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()

				return CLI{
					Pattern: "**",
					Output:  filepath.Join(tmpDir, "output.xml"),
				}
			},
			validate: func(t *testing.T, _ CLI) {
				t.Helper()
			},
			expectErr:   true,
			errContains: "missing mode header",
		},
		{
			name: "run with pattern non-recursive glob",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()

				return CLI{
					Pattern: "testdata/*.txt",
					Output:  filepath.Join(tmpDir, "output.xml"),
				}
			},
			validate: func(t *testing.T, cli CLI) {
				t.Helper()
				assert.FileExists(t, cli.Output)
			},
		},
		{
			name: "run with conversion failure",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()
				invalidFile := filepath.Join(tmpDir, "invalid.txt")
				err := os.WriteFile(invalidFile, []byte("invalid data"), 0644)
				require.NoError(t, err)

				return CLI{File: invalidFile, Output: "-"}
			},
			expectErr:   true,
			errContains: "conversion failed",
		},
		{
			name: "run with nonexistent file",
			setup: func(t *testing.T) CLI {
				t.Helper()

				return CLI{File: "nonexistent-file.txt", Output: "-"}
			},
			expectErr:   true,
			errContains: "failed to open input file",
		},
		{
			name: "run with pattern no match",
			setup: func(t *testing.T) CLI {
				t.Helper()

				return CLI{Pattern: "nonexistent-dir/**/*.out", Output: "-"}
			},
			expectErr:   true,
			errContains: "no files matched pattern",
		},
		{
			name: "run with pattern bad syntax",
			setup: func(t *testing.T) CLI {
				t.Helper()

				return CLI{Pattern: "invalid[pattern", Output: "-"}
			},
			expectErr:   true,
			errContains: "failed to process pattern",
		},
		{
			name: "run with output create error",
			setup: func(t *testing.T) CLI {
				t.Helper()

				return CLI{File: os.DevNull, Output: "/invalid-dir-path/output.xml"}
			},
			expectErr:   true,
			errContains: "failed to create output file",
		},
		{
			name: "run with pattern unreadable file",
			setup: func(t *testing.T) CLI {
				t.Helper()
				tmpDir := t.TempDir()
				validFile := filepath.Join(tmpDir, "1_valid.out")
				err := os.WriteFile(validFile, []byte("mode: set\n"), 0644)
				require.NoError(t, err)
				unreadableFile := filepath.Join(tmpDir, "2_unreadable.out")
				err = os.WriteFile(unreadableFile, []byte("mode: set\n"), 0000)
				require.NoError(t, err)

				return CLI{Pattern: filepath.Join(tmpDir, "*.out"), Output: "-"}
			},
			expectErr:   true,
			errContains: "failed to open matched file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cli := tt.setup(t)
			err := cli.Run()

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}

			if tt.validate != nil {
				tt.validate(t, cli)
			}
		})
	}
}

func TestFindMatchingFiles_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup       func(t *testing.T)
		name        string
		pattern     string
		errContains string
	}{
		{
			name:        "bad syntax",
			pattern:     "invalid[pattern",
			errContains: "syntax error",
		},
		{
			name:    "walk error",
			pattern: "/**/*.out",
			setup: func(t *testing.T) {
				t.Helper()
				// To simulate walk error we can use an unreadable directory
			},
			errContains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.name == "walk error" {
				tmpDir := t.TempDir()
				subDir := filepath.Join(tmpDir, "sub")
				err := os.Mkdir(subDir, 0000)
				require.NoError(t, err)
				tt.pattern = tmpDir + "/**/*.out"
			}

			_, err := findMatchingFiles(tt.pattern)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestIsMatched(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		base     string
		pattern  string
		expected bool
	}{
		{
			name:     "match slash pattern",
			path:     "base/sub/file.txt",
			base:     "sub/file.txt",
			pattern:  "sub/*.txt",
			expected: true,
		},
		{
			name:     "no match slash pattern",
			path:     "base/sub/file.txt",
			base:     "sub/file.txt",
			pattern:  "sub/*.out",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			matched := isMatched(tt.path, tt.base, tt.pattern)
			assert.Equal(t, tt.expected, matched)
		})
	}
}

func TestRunMain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		expectedCode int
	}{
		{
			name:         "successful run",
			args:         []string{"-f", "testdata/testdata_set.txt", "-o", os.DevNull},
			expectedCode: 0,
		},
		{
			name:         "invalid flag",
			args:         []string{"--invalid-flag"},
			expectedCode: 1,
		},
		{
			name:         "help flag",
			args:         []string{"--help"},
			expectedCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code := runMain(tt.args)
			assert.Equal(t, tt.expectedCode, code)
		})
	}
}
