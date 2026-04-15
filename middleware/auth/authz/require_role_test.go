package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/authorization"
)

func TestRequireRoleReturnsUnauthorizedWithoutAuthenticatedActor(t *testing.T) {
	mw := NewRequireRoleMiddleware("admin", func(_ context.Context) []string { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRoleReturnsForbiddenForAuthenticatedActorWithoutRole(t *testing.T) {
	mw := NewRequireRoleMiddleware("admin", func(_ context.Context) []string { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(authorization.WithActor(req.Context(), authorization.Actor{UserID: "user-1"}))
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
