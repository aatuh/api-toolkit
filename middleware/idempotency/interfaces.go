package idempotency

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// State describes the lifecycle of a stored idempotency record.
type State int

const (
	// StateUnknown indicates no stored state.
	StateUnknown State = iota
	// StateInFlight indicates a request is being processed.
	StateInFlight
	// StateCompleted indicates a request has a stored response.
	StateCompleted
	// StateAmbiguous indicates a request may have completed without a replay-safe response.
	StateAmbiguous
)

// Record captures request and response material for idempotent retries.
type Record struct {
	State            State
	RequestHash      string
	Status           int
	Header           http.Header
	Body             []byte
	CreatedAt        time.Time
	ReservationToken string
}

// Store persists idempotency records.
type Store interface {
	Get(ctx context.Context, key string) (Record, bool, error)
	TryBegin(ctx context.Context, key string, record Record, ttl time.Duration) (bool, error)
	Save(ctx context.Context, key string, record Record, ttl time.Duration) error
}

// ReservationReleaser removes an in-flight reservation using its token.
type ReservationReleaser interface {
	ReleaseReservation(ctx context.Context, key string, token string) error
}

// ReleasableStore combines persistence and token-aware release semantics.
type ReleasableStore interface {
	Store
	ReservationReleaser
}

var (
	// ErrLegacyInFlightReservationMissingToken reports mixed-version recovery without a token.
	ErrLegacyInFlightReservationMissingToken = errors.New("idempotency reservation token is missing from legacy in-flight record")
	// ErrLegacyInFlightTokenMismatch reports a stale legacy reservation token.
	ErrLegacyInFlightTokenMismatch = errors.New("idempotency reservation token mismatch")
)
