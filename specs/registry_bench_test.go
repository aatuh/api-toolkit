package specs

import (
	"fmt"
	"net/http"
	"testing"
)

func BenchmarkRegistryOpenAPI100Operations(b *testing.B) {
	registry := NewRegistry(Info{Title: "Widget API", Version: "1.0.0"})
	registry.SetServers([]Server{{URL: "https://api.example.test", Description: "production"}})
	registry.RegisterSecurityScheme("ApiKeyAuth", SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	registry.RegisterSchema("Widget", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "string"},
			"name": map[string]any{"type": "string"},
		},
	})
	for i := 0; i < 100; i++ {
		registry.Register(Operation{
			OperationID: fmt.Sprintf("listWidgets%d", i),
			Method:      http.MethodGet,
			Path:        fmt.Sprintf("/tenants/{tenant_id}/widgets/%03d", i),
			Summary:     "List widgets",
			Tags:        []string{"widgets"},
			Parameters: []Parameter{
				{Name: "tenant_id", In: "path", Required: true},
				{Name: "limit", In: "query", Schema: map[string]any{"type": "integer"}},
			},
			Security: []SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:read"}}},
			Responses: map[int]Response{
				http.StatusOK: {
					Description: "OK",
					Content: map[string]MediaType{
						"application/json": {SchemaRef: "#/components/schemas/Widget"},
					},
				},
			},
			Extensions: map[string]any{"x-tenant-scoped": true},
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := registry.OpenAPI()
		if err != nil {
			b.Fatalf("OpenAPI() error = %v", err)
		}
		if len(doc) == 0 {
			b.Fatal("OpenAPI() returned empty document")
		}
	}
}
