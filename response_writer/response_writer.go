//revive:disable:var-naming

package response_writer

import (
	"net/http"

	"github.com/aatuh/api-toolkit/httpx"
)

// WriteJSON writes a JSON response.
// Deprecated: use httpx.WriteJSON instead.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	httpx.WriteJSON(w, code, v)
}

// WriteErr writes a Problem+JSON error response.
// Deprecated: use httpx.WriteProblem or httpx.WriteError instead.
func WriteErr(w http.ResponseWriter, code int, msg string) {
	httpx.WriteProblem(w, code, httpx.Problem{
		Title:  http.StatusText(code),
		Detail: msg,
	})
}
