package oauth2

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/specs"
)

func TestRequireScopesAndBearerToken(t *testing.T) {
	claims := TokenClaims{Subject: "user_1", Scopes: []string{"widgets:read widgets:write"}}
	if err := RequireScopes(claims, "widgets:read", "widgets:write"); err != nil {
		t.Fatalf("RequireScopes() error = %v", err)
	}
	if err := RequireScopes(claims, "admin"); err == nil {
		t.Fatalf("expected missing scope")
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer token")
	if token, ok := BearerToken(request); !ok || token != "token" {
		t.Fatalf("BearerToken() = %q, %v", token, ok)
	}
}

func TestSecuritySchemeRegistration(t *testing.T) {
	registry := specs.NewRegistry(specs.Info{Title: "API", Version: "v1"})
	RegisterSecurityScheme(registry, "OAuth2", SecurityScheme("widgets:read"))
	body, err := registry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	if len(body) == 0 {
		t.Fatalf("OpenAPI body is empty")
	}
}
