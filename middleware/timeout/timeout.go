package timeout

import (
	"context"
	"net/http"
	"time"
)

// Middleware applies a per-request context timeout without writing responses.
type Middleware struct {
	Timeout time.Duration
}

// New constructs a timeout middleware with the given duration.
func New(d time.Duration) *Middleware { return &Middleware{Timeout: d} }

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with a context timeout.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.Timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
