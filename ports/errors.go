package ports

import "errors"

// ErrResourceMissing signals that a requested external resource was not found.
var ErrResourceMissing = errors.New("resource missing")

// ErrLegacyInFlightReservationMissingToken indicates a legacy in-flight idempotency
// reservation was recovered from mixed-version operation without a reservation token.
var ErrLegacyInFlightReservationMissingToken = errors.New("idempotency reservation token is missing from legacy in-flight record")

// ErrLegacyInFlightTokenMismatch indicates a legacy in-flight idempotency reservation
// was rejected due to token mismatch.
var ErrLegacyInFlightTokenMismatch = errors.New("idempotency reservation token mismatch")
