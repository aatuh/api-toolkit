package ports

import (
	"context"
	"net/http"
	"time"
)

// IdempotencyState describes the lifecycle of a stored idempotency record.
type IdempotencyState int

const (
	// IdempotencyStateUnknown indicates no stored state.
	IdempotencyStateUnknown IdempotencyState = iota
	// IdempotencyStateInFlight indicates a request is being processed.
	IdempotencyStateInFlight
	// IdempotencyStateCompleted indicates a request has a stored response.
	IdempotencyStateCompleted
	// IdempotencyStateAmbiguous indicates a request may have completed, but its
	// replay-safe response state could not be persisted.
	IdempotencyStateAmbiguous
)

// IdempotencyRecord captures request/response material for idempotent retries.
type IdempotencyRecord struct {
	State            IdempotencyState
	RequestHash      string
	Status           int
	Header           http.Header
	Body             []byte
	CreatedAt        time.Time
	ReservationToken string
}

// IdempotencyStore persists idempotency records (Redis/DB/etc).
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (IdempotencyRecord, bool, error)
	TryBegin(ctx context.Context, key string, record IdempotencyRecord, ttl time.Duration) (bool, error)
	Save(ctx context.Context, key string, record IdempotencyRecord, ttl time.Duration) error
}

// IdempotencyReleaser removes an in-flight reservation or stale record.
//
// Release keeps the v2-compatible source contract. It should be a no-op when
// the key is missing, expired, completed, or ambiguous, and may remove an
// in-flight record by key without checking ReservationToken. New stores should
// also implement IdempotencyReservationReleaser so middleware can use token-aware
// cleanup for current in-flight reservations.
//
// The idempotency middleware requires release semantics so non-stored outcomes
// can reopen a key safely for later retries without deleting completed replay
// records or ambiguous safety records.
type IdempotencyReleaser interface {
	Release(ctx context.Context, key string) error
}

// IdempotencyReservationReleaser removes an in-flight reservation using the
// reservation token recorded by the middleware.
//
// ReleaseReservation must be a no-op when the key is missing, expired,
// completed, or ambiguous. It must delete only the current in-flight record when
// token matches ReservationToken. Distributed stores must make the
// compare-and-delete operation atomic so a stale releaser cannot delete a newer
// reservation that replaced an expired or otherwise interleaved record. It must
// preserve tokened in-flight records when token does not match and return
// ErrLegacyInFlightTokenMismatch. Legacy tokenless in-flight records may be
// deleted only when callers also pass an empty token; implementations should
// return ErrLegacyInFlightReservationMissingToken after deleting that legacy
// record so mixed-version recovery paths can emit compatibility telemetry.
// Passing a non-empty token for a legacy tokenless record must preserve the
// record and return ErrLegacyInFlightTokenMismatch. Malformed records should be
// preserved and reported as decode errors.
type IdempotencyReservationReleaser interface {
	ReleaseReservation(ctx context.Context, key string, token string) error
}

// ReleasableIdempotencyStore combines persistence and release semantics for
// idempotent request processing.
type ReleasableIdempotencyStore interface {
	IdempotencyStore
	IdempotencyReleaser
}

// ReservationReleasableIdempotencyStore combines persistence, legacy release,
// and token-aware release semantics for idempotent request processing.
type ReservationReleasableIdempotencyStore interface {
	ReleasableIdempotencyStore
	IdempotencyReservationReleaser
}
