package ports

import "errors"

// ErrResourceMissing signals that a requested external resource was not found.
var ErrResourceMissing = errors.New("resource missing")
