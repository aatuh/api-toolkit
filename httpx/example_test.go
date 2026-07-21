package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aatuh/api-toolkit/v4/httpx"
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

func ExampleWriteJSONChecked() {
	rec := httptest.NewRecorder()

	err := httpx.WriteJSONChecked(rec, http.StatusCreated, map[string]string{"status": "created"})

	fmt.Println(err)
	fmt.Println(rec.Code)
	// Output:
	// <nil>
	// 201
}

func ExampleResponseWriteError() {
	err := &httpx.ResponseWriteError{Stage: httpx.ResponseWriteStageBody}

	fmt.Println(err)
	// Output:
	// http response body failed
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

func ExampleWriteProblemChecked() {
	rec := httptest.NewRecorder()

	err := httpx.WriteProblemChecked(rec, http.StatusBadRequest, httpx.Problem{
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "validation failed",
	})

	fmt.Println(err)
	fmt.Println(rec.Code)
	// Output:
	// <nil>
	// 400
}
