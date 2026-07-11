package routecontracts

import (
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v4/specs"
)

func BenchmarkRouteContractsRegisterAndValidate(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	operation := specs.Operation{
		OperationID: "listWidgets",
		Summary:     "List widgets",
		Tags:        []string{"widgets"},
		Responses: map[int]specs.Response{
			http.StatusOK: {Description: "OK"},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router := &fakeRouter{}
		specRegistry := specs.NewRegistry(specs.Info{Title: "Bench", Version: "1"})
		registry := NewRegistry(router, specRegistry)
		if err := registry.Get("/widgets", operation, handler); err != nil {
			b.Fatalf("Get() error = %v", err)
		}
		if err := registry.Validate(); err != nil {
			b.Fatalf("Validate() error = %v", err)
		}
	}
}
