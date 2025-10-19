package recover

import (
	"net/http"

	"github.com/aatuh/api-toolkit/httpx"
)

// Middleware converts panics into RFC-7807 problem+json responses.
// It intentionally does not leak panic values to clients.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
						Title:  http.StatusText(http.StatusInternalServerError),
						Detail: "internal server error",
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
