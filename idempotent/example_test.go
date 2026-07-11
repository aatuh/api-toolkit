package idempotent_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v4/idempotent"
)

func ExampleRequireKey() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/charges", nil)
	req.Header.Set("Idempotency-Key", "charge-123")

	key, err := idempotent.RequireKey(req, "Idempotency-Key")
	if err != nil {
		panic(err)
	}

	fmt.Println(key)

	// Output:
	// charge-123
}
