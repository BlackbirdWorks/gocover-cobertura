package cobertura_test

import (
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
	"github.com/stretchr/testify/assert"
)

func TestClass_HitRate_Zero(t *testing.T) {
	t.Parallel()
	c := cobertura.Class{}
	assert.InDelta(t, float32(0.0), c.HitRate(), 0.0001)
}
