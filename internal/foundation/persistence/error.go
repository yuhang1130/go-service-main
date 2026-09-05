package persistence

import (
	"errors"
	"fmt"
)

// ErrConflict marks a write rejected by a persistence constraint. Application
// services can map it to a stable conflict response without depending on a
// database driver or adapter package.
var ErrConflict = errors.New("persistence conflict")

func Conflict(cause error) error {
	if cause == nil {
		return ErrConflict
	}
	return fmt.Errorf("%w: %w", ErrConflict, cause)
}
