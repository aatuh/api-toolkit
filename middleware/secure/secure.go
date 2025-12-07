package secure

import (
	"net/http"

	"github.com/aatuh/api-toolkit/ports"
)

// Handler adds a minimal set of sane security headers.
// Safe for local dev; HSTS is only set when TLS is detected.
type Handler struct{}

func New() ports.Middleware { return &Handler{} }

func (h *Handler) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defaultCSP := "default-src 'none'; frame-ancestors 'none'"
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security",
					"max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
			if w.Header().Get("Content-Security-Policy") == "" {
				w.Header().Set("Content-Security-Policy", defaultCSP)
			}
		})
	}
}
