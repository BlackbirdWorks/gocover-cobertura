package errs_test

import (
	"errors"
	"testing"

	"github.com/blackbirdworks/gocover-cobertura/pkg/errs"
	"github.com/stretchr/testify/require"
)

var (
	errBase  = errors.New("base error")
	errInner = errors.New("inner error")
)

func TestWrap(t *testing.T) {
	t.Parallel()

	// Test wrapping
	wrapped := errs.Wrap(errInner, errBase)
	require.Error(t, wrapped)
	require.ErrorIs(t, wrapped, errBase)
	require.ErrorIs(t, wrapped, errInner)
	require.Equal(t, "base error: inner error", wrapped.Error())

	// Test nil error
	nilErr := errs.Wrap(nil, errBase)
	require.NoError(t, nilErr)
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	// Test wrapping with format args
	wrapped := errs.Wrapf(errInner, errBase, "failed to %s", "do thing")
	require.Error(t, wrapped)
	require.ErrorIs(t, wrapped, errBase)
	require.ErrorIs(t, wrapped, errInner)
	require.Equal(t, "base error: inner error failed to do thing", wrapped.Error())

	// Test nil error
	nilErr := errs.Wrapf(nil, errBase, "failed to do thing")
	require.NoError(t, nilErr)
}
