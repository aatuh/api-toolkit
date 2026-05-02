package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

func ExampleWriteJSON() {
	rec := httptest.NewRecorder()

	httpx.WriteJSON(rec, http.StatusAccepted, map[string]string{"status": "ok"})

	fmt.Println(rec.Code)
	fmt.Println(strings.TrimSpace(rec.Body.String()))
	// Output:
	// 202
	// {"status":"ok"}
}

func ExampleWriteProblem() {
	rec := httptest.NewRecorder()

	httpx.WriteProblem(rec, http.StatusBadRequest, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "validation failed",
	})

	fmt.Println(rec.Code)
	fmt.Println(rec.Header().Get("Content-Type"))
	// Output:
	// 400
	// application/problem+json
}
