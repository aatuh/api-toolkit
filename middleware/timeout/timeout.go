package timeout

import (
	"net/http"
	"time"
)

type Middleware struct {
	Timeout time.Duration
}

func New(d time.Duration) *Middleware { return &Middleware{Timeout: d} }

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, m.Timeout, "request timeout")
}
