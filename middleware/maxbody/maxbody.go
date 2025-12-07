package maxbody

import "net/http"

type Middleware struct {
	MaxBytes int64
}

func New(max int64) *Middleware { return &Middleware{MaxBytes: max} }

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.MaxBytes > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, m.MaxBytes)
		}
		next.ServeHTTP(w, r)
	})
}
