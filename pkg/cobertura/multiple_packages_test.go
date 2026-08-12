package cobertura_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_MultiplePackagesSort(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create package A
	pkgADir := filepath.Join(tmpDir, "pkgA")
	require.NoError(t, os.MkdirAll(pkgADir, 0755))
	fileA := filepath.Join(pkgADir, "a.go")
	require.NoError(t, os.WriteFile(fileA, []byte("package pkgA\n"), 0644))

	// Create package B
	pkgBDir := filepath.Join(tmpDir, "pkgB")
	require.NoError(t, os.MkdirAll(pkgBDir, 0755))
	fileB := filepath.Join(pkgBDir, "b.go")
	require.NoError(t, os.WriteFile(fileB, []byte("package pkgB\n"), 0644))

	inStr := "mode: set\n" + fileA + ":1.1,2.2 1 1\n" + fileB + ":1.1,2.2 1 1\n"

	p := cobertura.NewParser()
	cov, err := p.Parse(strings.NewReader(inStr))
	require.NoError(t, err)

	require.Len(t, cov.Packages, 2)
	assert.True(t, strings.HasSuffix(cov.Packages[0].Name, "pkgA"))
	assert.True(t, strings.HasSuffix(cov.Packages[1].Name, "pkgB"))
}
