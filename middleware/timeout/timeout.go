package timeout

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Propagator applies a per-request context deadline without writing timeout
// responses. Handlers and downstream calls must observe ctx.Done for the
// deadline to take effect.
type Propagator struct {
	Timeout time.Duration
}

// Middleware is kept as a backward-compatible alias for Propagator.
type Middleware = Propagator

// Options configures the timeout middleware.
type Options struct {
	Timeout time.Duration
}

// NewPropagator constructs a cooperative request-deadline propagator with the
// given duration.
func NewPropagator(opts Options) (*Propagator, error) {
	if opts.Timeout <= 0 {
		return nil, errors.New("timeout must be greater than zero")
	}
	return &Propagator{Timeout: opts.Timeout}, nil
}

// New constructs a cooperative request-deadline propagator.
// Deprecated: use NewPropagator to make the non-aborting behavior explicit.
func New(opts Options) (*Propagator, error) {
	return NewPropagator(opts)
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Propagator) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with a context deadline.
// It does not abort writes or synthesize timeout responses.
func (m *Propagator) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.Timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
