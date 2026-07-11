package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/authorization"
)

func TestRequireRoleReturnsUnauthorizedWithoutAuthenticatedActor(t *testing.T) {
	mw := mustRequireRoleMiddleware(t, "admin", func(_ context.Context) []string { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRoleReturnsForbiddenForAuthenticatedActorWithoutRole(t *testing.T) {
	mw := mustRequireRoleMiddleware(t, "admin", func(_ context.Context) []string { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(authorization.WithActor(req.Context(), authorization.Actor{UserID: "user-1"}))
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestValidateRequireRoleMiddlewareAcceptsValidConfiguration(t *testing.T) {
	mw := mustRequireRoleMiddleware(t, "admin", func(_ context.Context) []string { return []string{"admin"} })
	if err := ValidateRequireRoleMiddleware("GET", "/admin", mw); err != nil {
		t.Fatalf("expected validation to pass: %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesAcceptsValidConfiguration(t *testing.T) {
	mwAdmin := mustRequireRoleMiddleware(t, "admin", func(_ context.Context) []string { return []string{"admin"} })
	mwOps := mustRequireRoleMiddleware(t, "ops", func(_ context.Context) []string { return []string{"ops"} })

	if err := ValidateRequireRoleMiddlewareRoutes([]RequireRoleRouteSpec{
		{Method: "GET", Route: "/admin", Middleware: mwAdmin},
		{Method: "POST", Route: "/ops", Middleware: mwOps},
	}); err != nil {
		t.Fatalf("expected route batch validation to pass: %v", err)
	}
}

func TestNewRequireRoleMiddlewareValidatesConfiguration(t *testing.T) {
	mw, err := NewRequireRoleMiddleware(" admin ", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("expected valid middleware: %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware")
	}
	if err := ValidateRequireRoleMiddleware("GET", "/admin", mw); err != nil {
		t.Fatalf("expected valid middleware: %v", err)
	}
}

func TestNewRequireRoleMiddlewareRejectsInvalidConfig(t *testing.T) {
	if _, err := NewRequireRoleMiddleware("   ", nil); err == nil {
		t.Fatal("expected invalid middleware config error")
	}
}

func TestNewRequireRoleMiddlewareCheckedRejectsEmptyRole(t *testing.T) {
	if _, err := NewRequireRoleMiddlewareChecked("   ", func(_ context.Context) []string { return []string{"admin"} }); err == nil {
		t.Fatal("expected empty role constructor error")
	}
}

func TestNewRequireRoleMiddlewareCheckedRejectsNilRoleResolver(t *testing.T) {
	if _, err := NewRequireRoleMiddlewareChecked("admin", nil); err == nil {
		t.Fatal("expected nil resolver constructor error")
	}
}

func TestNewRequireRoleMiddlewareCheckedAcceptsValidConfiguration(t *testing.T) {
	mw, err := NewRequireRoleMiddlewareChecked("admin", func(_ context.Context) []string { return []string{"admin"} })
	if err != nil {
		t.Fatalf("expected checked constructor to pass: %v", err)
	}
	if mw == nil {
		t.Fatal("expected middleware")
	}
}

func TestValidateRequireRoleMiddlewareRejectsNilRoleResolver(t *testing.T) {
	mw := &RequireRoleMiddleware{
		role:         "admin",
		rolesFromCtx: nil,
	}
	if err := ValidateRequireRoleMiddleware("GET", "/admin", mw); err == nil {
		t.Fatal("expected nil resolver validation error")
	}
}

func TestValidateRequireRoleMiddlewareRejectsEmptyRoleInRouteContext(t *testing.T) {
	mw := &RequireRoleMiddleware{
		role:         "   ",
		rolesFromCtx: func(_ context.Context) []string { return []string{"admin"} },
	}
	err := ValidateRequireRoleMiddleware("POST", "/admin", mw)
	if err == nil {
		t.Fatal("expected empty role validation error")
	}
	if !errors.Is(err, ErrRequireRoleMissingRole) {
		t.Fatalf("expected ErrRequireRoleMissingRole, got %v", err)
	}
	if !strings.Contains(err.Error(), "POST /admin") {
		t.Fatalf("expected actionable route context in validation error, got %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRejectsNilMiddleware(t *testing.T) {
	err := ValidateRequireRoleMiddleware("GET", "/admin", nil)
	if err == nil {
		t.Fatal("expected nil middleware validation error")
	}
	if !strings.Contains(err.Error(), "middleware is nil") {
		t.Fatalf("expected middleware context error, got %v", err)
	}
	if !strings.Contains(err.Error(), "GET /admin") {
		t.Fatalf("expected actionable route context in validation error, got %v", err)
	}
	if !strings.Contains(err.Error(), "for route") {
		t.Fatalf("expected wrapped validation context shape, got %v", err)
	}
}

func TestValidateRequireRoleMiddlewareRoutesRejectsInvalidEntry(t *testing.T) {
	mwAdmin := mustRequireRoleMiddleware(t, "admin", func(_ context.Context) []string { return []string{"admin"} })
	err := ValidateRequireRoleMiddlewareRoutes([]RequireRoleRouteSpec{
		{Method: "GET", Route: "/admin", Middleware: mwAdmin},
		{Method: "POST", Route: "/billing", Middleware: &RequireRoleMiddleware{role: "   ", rolesFromCtx: func(_ context.Context) []string { return []string{"admin"} }}},
		{Method: "PUT", Route: "/ops", Middleware: &RequireRoleMiddleware{role: "ops", rolesFromCtx: nil}},
	})
	if err == nil {
		t.Fatal("expected validation errors in route batch")
	}
	if !strings.Contains(err.Error(), "POST /billing") {
		t.Fatalf("expected invalid route in error output, got %v", err)
	}
	if !strings.Contains(err.Error(), "PUT /ops") {
		t.Fatalf("expected second invalid route in error output, got %v", err)
	}
	if !strings.Contains(err.Error(), "for route") {
		t.Fatalf("expected wrapped validation context shape, got %v", err)
	}
}

func mustRequireRoleMiddleware(t *testing.T, role string, rolesFromCtx RolesFromContext) *RequireRoleMiddleware {
	t.Helper()
	mw, err := NewRequireRoleMiddleware(role, rolesFromCtx)
	if err != nil {
		t.Fatalf("new require role middleware: %v", err)
	}
	return mw
}
