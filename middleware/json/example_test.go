package jsonmw_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	jsonmw "github.com/aatuh/api-toolkit/v4/middleware/json"
)

func ExampleStrictDecoder() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", strings.NewReader(`{"name":"starter"}`))

	decoder, err := jsonmw.StrictDecoder(req)
	if err != nil {
		panic(err)
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := decoder.Decode(&payload); err != nil {
		panic(err)
	}

	fmt.Println(payload.Name)

	// Output:
	// starter
}
