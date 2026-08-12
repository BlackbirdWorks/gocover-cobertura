package cobertura_test

import (
	"testing"

	cobertura "github.com/blackbirdworks/gocover-cobertura/pkg/cobertura"
	"github.com/stretchr/testify/assert"
)

func TestCoverageLine_UnmarshalText_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"no spaces", "nospacesatall"},
		{"one space", "onespace here"},
		{"no comma", "file.go:1.1 1 1"},
		{"no start dot", "file.go:1,2.2 1 1"},
		{"no end dot", "file.go:1.1,2 1 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var c cobertura.CoverageLine
			err := c.UnmarshalText([]byte(tt.input))
			assert.ErrorIs(t, err, cobertura.ErrBadFormat)
		})
	}
}
