package cobertura_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
)

func TestClass_HitRate_Zero(t *testing.T) {
	t.Parallel()

	c := cobertura.Class{}
	assert.InDelta(t, float32(0.0), c.HitRate(), 0.0001)
}

func TestCoverage_HitRate(t *testing.T) {
	t.Parallel()

	empty := &cobertura.Coverage{}
	assert.InDelta(t, float32(0.0), empty.HitRate(), 0.0001)

	cov := &cobertura.Coverage{
		Packages: []*cobertura.Package{
			{
				Classes: []*cobertura.Class{
					{
						Methods: []*cobertura.Method{
							{
								Lines: cobertura.Lines{
									{Number: 1, Hits: 1},
									{Number: 2, Hits: 0},
								},
							},
						},
					},
				},
			},
		},
	}
	assert.InDelta(t, float32(0.5), cov.HitRate(), 0.0001)
}
