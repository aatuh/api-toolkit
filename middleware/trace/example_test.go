package trace_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/middleware/trace"
)

type exampleIDGen struct {
	value string
}

func (g exampleIDGen) New() string { return g.value }

func ExampleNew() {
	middleware, err := trace.New(trace.Options{
		TraceIDGen: exampleIDGen{value: "11111111111111111111111111111111"},
		SpanIDGen:  exampleIDGen{value: "2222222222222222"},
	})
	if err != nil {
		panic(err)
	}
	var gotTraceID string
	handler := middleware.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceID = trace.GetTraceID(r)
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets", nil)
	handler.ServeHTTP(recorder, req)

	fmt.Println(gotTraceID)

	// Output:
	// 11111111111111111111111111111111
}
