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
