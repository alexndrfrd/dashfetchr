package ports

import "errors"

// ErrNotFound is returned when a repository lookup misses.
var ErrNotFound = errors.New("not found")
