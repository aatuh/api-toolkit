package binding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type benchWidget struct {
	Name     string   `json:"name" query:"name" required:"true"`
	Quantity int      `json:"quantity" query:"quantity" required:"true"`
	Active   bool     `json:"active" query:"active"`
	Tags     []string `query:"tag"`
}

func BenchmarkBindingDecodeJSON(b *testing.B) {
	const body = `{"name":"starter","quantity":2,"active":true}`

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
		got, err := DecodeJSON[benchWidget](req, JSONConfig{MaxBytes: 1 << 20})
		if err != nil {
			b.Fatalf("DecodeJSON() error = %v", err)
		}
		if got.Name == "" || got.Quantity == 0 {
			b.Fatalf("DecodeJSON() decoded empty required fields: %#v", got)
		}
	}
}

func BenchmarkBindingDecodeQuery(b *testing.B) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?name=starter&quantity=2&active=true&tag=core&tag=docs", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := DecodeQuery[benchWidget](req, QueryConfig{})
		if err != nil {
			b.Fatalf("DecodeQuery() error = %v", err)
		}
		if got.Name == "" || got.Quantity == 0 || len(got.Tags) != 2 {
			b.Fatalf("DecodeQuery() decoded unexpected fields: %#v", got)
		}
	}
}
