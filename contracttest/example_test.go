package contracttest_test

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v3/contracttest"
	"github.com/aatuh/api-toolkit/v3/specs"
)

func ExampleAssertOperationHasResponse() {
	registry := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "1.0.0"})
	registry.Register(specs.Operation{
		Method: http.MethodGet,
		Path:   "/widgets",
		Responses: map[int]specs.Response{
			http.StatusOK: {Description: "OK"},
		},
	})

	var t testing.T
	contracttest.AssertOperationHasResponse(&t, registry, http.MethodGet, "/widgets", http.StatusOK)
}
