package oauth2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestValidatorFuncAndClaimMappings(t *testing.T) {
	if _, err := (ValidatorFunc(nil)).ValidateToken(context.Background(), "token"); err == nil {
		t.Fatal("expected nil validator error")
	}
	want := TokenClaims{Subject: "user_1", TenantID: "tenant_1"}
	validator := ValidatorFunc(func(ctx context.Context, token string) (TokenClaims, error) {
		if token != "token" {
			return TokenClaims{}, errors.New("bad token")
		}
		return want, nil
	})
	got, err := validator.ValidateToken(context.Background(), "token")
	if err != nil || got.Subject != want.Subject {
		t.Fatalf("ValidateToken() = %#v, %v", got, err)
	}
	if actor := got.Actor(); actor.UserID != "user_1" {
		t.Fatalf("Actor() = %#v", actor)
	}
	if scope := got.AuthorizationScope(); scope.UserID != "user_1" || scope.TenantID != "tenant_1" {
		t.Fatalf("AuthorizationScope() = %#v", scope)
	}
}

func TestScopeSetNormalizationAndRequirements(t *testing.T) {
	set := NewScopeSet(" read write ", "write", "", "admin")
	for _, scope := range []string{"read", "write", "admin"} {
		if !set.Has(scope) {
			t.Fatalf("scope %q missing from %#v", scope, set)
		}
	}
	if set.Has("missing") || set.Has(" ") {
		t.Fatalf("unexpected scope in %#v", set)
	}
	claims := TokenClaims{Scopes: []string{"read write"}}
	if err := RequireScopes(claims, "", "read"); err != nil {
		t.Fatalf("RequireScopes() error = %v", err)
	}
	if err := RequireScopes(claims, "admin"); err == nil {
		t.Fatal("expected missing required scope")
	}
}

func TestBearerTokenMalformedHeaders(t *testing.T) {
	tests := []string{"", "Basic abc", "Bearer", "Bearer    ", "bearer"}
	for _, header := range tests {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		if token, ok := BearerToken(req); ok || token != "" {
			t.Fatalf("BearerToken(%q) = %q, %v", header, token, ok)
		}
	}
	if token, ok := BearerToken(nil); ok || token != "" {
		t.Fatalf("BearerToken(nil) = %q, %v", token, ok)
	}
}

func TestSecuritySchemeDefaultsAndDeterministicScopes(t *testing.T) {
	scheme := SecurityScheme("write", "read")
	if scheme.Type != "http" || scheme.Scheme != "bearer" || scheme.BearerFormat != "JWT" {
		t.Fatalf("scheme = %#v", scheme)
	}
	if !reflect.DeepEqual(scheme.Flows["scopes"], []string{"read", "write"}) {
		t.Fatalf("scopes = %#v", scheme.Flows["scopes"])
	}
	RegisterSecurityScheme(nil, "", scheme)
}
