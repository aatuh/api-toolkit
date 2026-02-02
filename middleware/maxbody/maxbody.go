package maxbody

import (
	"errors"
	"net/http"
)

// Middleware enforces a maximum request body size.
type Middleware struct {
	MaxBytes int64
}

// Options configures the max-body middleware.
type Options struct {
	MaxBytes int64
}

// New constructs a max-body middleware with the given limit.
func New(opts Options) (*Middleware, error) {
	if opts.MaxBytes <= 0 {
		return nil, errors.New("max body bytes must be greater than zero")
	}
	return &Middleware{MaxBytes: opts.MaxBytes}, nil
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with body size limits.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.MaxBytes > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, m.MaxBytes)
		}
		next.ServeHTTP(w, r)
	})
}
