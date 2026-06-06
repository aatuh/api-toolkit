package apikey

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/authorization"
)

func TestHandlerAuthenticatesAuthorizationAPIKey(t *testing.T) {
	mw := newTestMiddleware(t, Principal{ID: "key_123", TenantID: "tenant_1", Scopes: []string{"write", "read"}})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "ApiKey secret")
	rec := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.ID != "key_123" {
			t.Fatalf("principal = %#v, %v", principal, ok)
		}
		actor, ok := authorization.ActorFromContext(r.Context())
		if !ok || actor.UserID != "key_123" {
			t.Fatalf("actor = %#v, %v", actor, ok)
		}
		if tenant, ok := authorization.TenantIDFromContext(r.Context()); !ok || tenant != "tenant_1" {
			t.Fatalf("tenant = %q, %v", tenant, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestHandlerAuthenticatesXAPIKey(t *testing.T) {
	mw := newTestMiddleware(t, Principal{ID: "key_123"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestHandlerRejectsMissingMalformedAndInvalidKeys(t *testing.T) {
	mw := newTestMiddleware(t, Principal{ID: "key_123"})
	tests := []struct {
		name   string
		header func(http.Header)
		want   int
		body   string
	}{
		{name: "missing", want: http.StatusUnauthorized, body: "API key required"},
		{name: "malformed authorization", header: func(h http.Header) {
			h.Set("Authorization", "ApiKey")
		}, want: http.StatusBadRequest, body: "invalid API key credentials"},
		{name: "multiple", header: func(h http.Header) {
			h.Set("Authorization", "ApiKey secret")
			h.Set("X-API-Key", "secret")
		}, want: http.StatusBadRequest, body: "invalid API key credentials"},
		{name: "duplicate authorization api keys", header: func(h http.Header) {
			h.Add("Authorization", "ApiKey secret")
			h.Add("Authorization", "ApiKey other")
		}, want: http.StatusBadRequest, body: "invalid API key credentials"},
		{name: "invalid", header: func(h http.Header) {
			h.Set("X-API-Key", "wrong")
		}, want: http.StatusUnauthorized, body: "invalid API key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			if tt.header != nil {
				tt.header(req.Header)
			}
			rec := httptest.NewRecorder()
			mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("next should not run")
			})).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Fatalf("body = %s, want %q", rec.Body.String(), tt.body)
			}
		})
	}
}

func TestHandlerRejectsDuplicateAuthorizationAPIKeys(t *testing.T) {
	mw := newTestMiddleware(t, Principal{ID: "key_123"})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Add("Authorization", "ApiKey secret")
	req.Header.Add("Authorization", "ApiKey other")
	rec := httptest.NewRecorder()

	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run for duplicate API key authorization headers")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid API key credentials") {
		t.Fatalf("body = %s, want invalid API key credentials", rec.Body.String())
	}
}

func TestOptionalHandlerAllowsMissingKeyButRejectsInvalidKey(t *testing.T) {
	mw := newTestMiddleware(t, Principal{ID: "key_123"})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFromContext(r.Context()); ok {
			t.Fatal("principal should not be present")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("missing status = %d, want 204", rec.Code)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "wrong")
	rec = httptest.NewRecorder()
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid status = %d, want 401", rec.Code)
	}
}

func TestRequireScopeMiddleware(t *testing.T) {
	handler := RequireScopeMiddleware("write")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(WithPrincipal(context.Background(), Principal{ID: "key", Scopes: []string{"read"}}), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(WithPrincipal(context.Background(), Principal{ID: "key", Scopes: []string{"write"}}), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestNewMiddlewareRequiresVerifier(t *testing.T) {
	if _, err := NewMiddleware(Config{}); err == nil {
		t.Fatal("expected verifier error")
	}
}

func newTestMiddleware(t *testing.T, principal Principal) *Middleware {
	t.Helper()
	mw, err := NewMiddleware(Config{
		Verifier: VerifierFunc(func(ctx context.Context, presented PresentedKey) (Principal, error) {
			if presented.Value != "secret" {
				return Principal{}, errors.New("not found")
			}
			return principal, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewMiddleware: %v", err)
	}
	return mw
}
