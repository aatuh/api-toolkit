package tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/authorization"
)

type staticParam struct {
	value string
}

func (s staticParam) URLParam(_ *http.Request, _ string) string {
	return s.value
}

func TestTenantMiddlewareRequiresTenant(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}

func TestTenantMiddlewareMismatch(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
		TenantFromContext: func(_ context.Context) (string, bool) {
			return "tenant-a", true
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}

func TestTenantMiddlewareSetsScope(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
		TenantFromContext: func(_ context.Context) (string, bool) {
			return "tenant-a", true
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope, ok := authorization.ScopeFromContext(r.Context())
		if !ok || scope.TenantID != "tenant-a" {
			t.Fatalf("expected tenant scope, got %+v (ok=%v)", scope, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d", rec.Code)
	}
}

func TestTenantMiddlewareURLParamMismatch(t *testing.T) {
	mw, err := New(Options{
		HeaderName:        "X-Tenant-ID",
		URLParam:          "tenant",
		URLParamExtractor: staticParam{value: "tenant-a"},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}

func TestTenantMiddlewareHeaderMultipleValues(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Add("X-Tenant-ID", "tenant-a")
	req.Header.Add("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}

func TestNewTenantRequiresSource(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error for missing sources")
	}
}

func TestNewTenantRequiresURLParamExtractor(t *testing.T) {
	if _, err := New(Options{URLParam: "tenant"}); err == nil {
		t.Fatal("expected error for missing url param extractor")
	}
}
