package errs_test

import (
	"errors"
	"testing"

	"github.com/blackbirdworks/gocover-cobertura/pkg/errs"
	"github.com/stretchr/testify/require"
)

func TestWrap(t *testing.T) {
	t.Parallel()
	err := errors.New("base error") //nolint:err113 // this is a test base error

	// Test wrapping with format args
	wrapped := errs.Wrap(err, "failed to %s", "do thing")
	require.Error(t, wrapped)
	require.Equal(t, "failed to do thing: base error", wrapped.Error())

	// Test wrapping without format args
	wrappedNoArgs := errs.Wrap(err, "failed to do thing")
	require.Error(t, wrappedNoArgs)
	require.Equal(t, "failed to do thing: base error", wrappedNoArgs.Error())

	// Test nil error
	nilErr := errs.Wrap(nil, "should be nil")
	require.NoError(t, nilErr)
}

func TestNew(t *testing.T) {
	t.Parallel()
	err := errs.New("something went %s", "wrong")
	require.Error(t, err)
	require.Equal(t, "something went wrong", err.Error())
}
