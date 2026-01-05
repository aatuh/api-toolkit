package identity

import "errors"

var (
	// ErrInvalid indicates validation failure.
	ErrInvalid = errors.New("invalid request")
	// ErrNotFound indicates no user matched query.
	ErrNotFound = errors.New("user not found")
	// ErrConflict indicates write conflicts such as duplicate identities.
	ErrConflict = errors.New("user conflict")
	// ErrInternal is used as a defensive fallback for unknown failures.
	ErrInternal = errors.New("internal error")
)
