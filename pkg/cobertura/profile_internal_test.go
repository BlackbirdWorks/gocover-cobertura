package cobertura

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveImportDir_CacheHit(t *testing.T) {
	t.Parallel()

	var cache sync.Map

	pkgPath := "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"

	dir1, err := resolveImportDir(&cache, pkgPath)
	require.NoError(t, err)
	assert.NotEmpty(t, dir1)

	dir2, err := resolveImportDir(&cache, pkgPath)
	require.NoError(t, err)
	assert.Equal(t, dir1, dir2)
}
