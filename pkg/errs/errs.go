package errs

import (
	"fmt"
)

// Wrap wraps an existing error with a base error.
// Both errors will be in the error chain.
func Wrap(err error, baseErr error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w", baseErr, err)
}

// Wrapf wraps an existing error with a base error and additional formatted context.
func Wrapf(err error, baseErr error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w %s", baseErr, err, fmt.Sprintf(format, args...))
}
