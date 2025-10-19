package timeout

import (
	"net/http"
	"time"
)

type Middleware struct {
	Timeout time.Duration
}

func New(d time.Duration) *Middleware { return &Middleware{Timeout: d} }

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, m.Timeout, "request timeout")
}
