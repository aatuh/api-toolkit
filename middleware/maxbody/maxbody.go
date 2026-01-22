package maxbody

import "net/http"

// Middleware enforces a maximum request body size.
type Middleware struct {
	MaxBytes int64
}

// New constructs a max-body middleware with the given limit.
func New(maxBytes int64) *Middleware { return &Middleware{MaxBytes: maxBytes} }

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with body size limits.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.MaxBytes > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, m.MaxBytes)
		}
		next.ServeHTTP(w, r)
	})
}
