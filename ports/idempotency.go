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
	State       IdempotencyState
	RequestHash string
	Status      int
	Header      http.Header
	Body        []byte
	CreatedAt   time.Time
}

// IdempotencyStore persists idempotency records (Redis/DB/etc).
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (IdempotencyRecord, bool, error)
	TryBegin(ctx context.Context, key string, record IdempotencyRecord, ttl time.Duration) (bool, error)
	Save(ctx context.Context, key string, record IdempotencyRecord, ttl time.Duration) error
}

// IdempotencyReleaser removes an in-flight reservation or stale record.
//
// The idempotency middleware requires release semantics so non-stored outcomes
// can reopen a key safely for later retries.
type IdempotencyReleaser interface {
	Release(ctx context.Context, key string) error
}

// ReleasableIdempotencyStore combines persistence and release semantics for
// idempotent request processing.
type ReleasableIdempotencyStore interface {
	IdempotencyStore
	IdempotencyReleaser
}
