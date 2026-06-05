package idempotency_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
)

func ExampleDefaultHash() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charges?id=1", nil)

	hash, err := idempotencymw.DefaultHash(req, []byte(`{"amount":42}`))
	if err != nil {
		panic(err)
	}

	fmt.Println(len(hash))

	// Output:
	// 64
}
