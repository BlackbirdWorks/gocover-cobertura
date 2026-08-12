package errs

import (
	"fmt"
)

// Wrap wraps an existing error with additional context.
// If err is nil, Wrap returns nil.
func Wrap(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("%s: %w", format, err)
	}

	return fmt.Errorf(format+": %w", append(args, err)...) //nolint:err113 // generic error wrapper
}

// New creates a new error with a formatted message.
func New(format string, args ...any) error {
	return fmt.Errorf(format, args...) //nolint:err113 // generic error wrapper
}
