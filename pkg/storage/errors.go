package storage

import "errors"

// ErrNotFound is returned when a requested object does not exist.
var ErrNotFound = errors.New("object not found")

// ErrAccessDenied is returned when the caller lacks permission.
var ErrAccessDenied = errors.New("access denied")

// IsNotFound reports whether err indicates a missing object.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
