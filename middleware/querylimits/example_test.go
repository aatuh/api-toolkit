package querylimits_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/middleware/querylimits"
)

func ExampleNew() {
	middleware, err := querylimits.New(querylimits.Options{MaxLimit: 10})
	if err != nil {
		panic(err)
	}
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets?limit=5", nil)
	handler.ServeHTTP(recorder, req)

	fmt.Println(recorder.Code)

	// Output:
	// 204
}
