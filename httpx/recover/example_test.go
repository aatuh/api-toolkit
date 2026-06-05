package recover_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	recovermw "github.com/aatuh/api-toolkit/v3/httpx/recover"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func ExampleNew() {
	middleware := recovermw.New(recovermw.WithLogger(ports.NopLogger{}), recovermw.WithStackLogging(false))
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(recorder, req)

	fmt.Println(recorder.Code)

	// Output:
	// 500
}
