package deprecation_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/aatuh/api-toolkit/v3/middleware/deprecation"
)

func ExampleNew() {
	mw, err := deprecation.New(deprecation.Config{
		DeprecatedAt: time.Unix(1_700_000_000, 0),
	})
	if err != nil {
		panic(err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/widgets", nil)
	handler.ServeHTTP(recorder, req)

	fmt.Println(recorder.Header().Get("Deprecation"))

	// Output:
	// @1700000000
}
