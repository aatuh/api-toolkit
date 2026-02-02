package httpclient

import (
	"context"
	"errors"
)

// Bulkhead limits concurrency for outbound requests.
type Bulkhead interface {
	Acquire(ctx context.Context) (func(), error)
}

// SemaphoreBulkhead bounds concurrent requests using a semaphore.
type SemaphoreBulkhead struct {
	ch chan struct{}
}

// NewSemaphoreBulkhead creates a semaphore bulkhead with the given limit.
func NewSemaphoreBulkhead(limit int) (*SemaphoreBulkhead, error) {
	if limit <= 0 {
		return nil, errors.New("bulkhead limit must be positive")
	}
	return &SemaphoreBulkhead{ch: make(chan struct{}, limit)}, nil
}

// Acquire reserves capacity until the release function is called.
func (b *SemaphoreBulkhead) Acquire(ctx context.Context) (func(), error) {
	if b == nil {
		return nil, errors.New("bulkhead is nil")
	}
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	select {
	case b.ch <- struct{}{}:
		return func() {
			<-b.ch
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
