package recover

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/response_writer"
)

// Middleware converts panics into RFC 9457 problem details responses.
// It intentionally does not leak panic values to clients.
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := response_writer.Wrap(w)
			defer func() {
				if rec := recover(); rec != nil {
					_, _ = fmt.Fprintf(os.Stderr, "panic recovered: %v\n%s\n", rec, debug.Stack())
					if ww.Committed() {
						return
					}
					httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
						Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
						Title:  http.StatusText(http.StatusInternalServerError),
						Detail: "internal server error",
					})
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
