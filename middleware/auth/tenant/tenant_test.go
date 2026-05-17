package tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v3/authorization"
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rec.Code)
	}
}

func TestTenantMiddlewareRequiresTenantForAuthenticatedActor(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req = req.WithContext(authorization.WithActor(req.Context(), authorization.Actor{UserID: "user-1"}))
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d", rec.Code)
	}
}

func TestTenantMiddlewareRequireAllSourcesRequiresHeaderAndContext(t *testing.T) {
	mw, err := New(Options{
		HeaderName: "X-Tenant-ID",
		TenantFromContext: func(ctx context.Context) (string, bool) {
			return authorization.TenantIDFromContext(ctx)
		},
		RequireAllSources: true,
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

	ctx := authorization.WithActor(context.Background(), authorization.Actor{UserID: "user-1"})
	ctx = authorization.WithScope(ctx, authorization.Scope{UserID: "user-1", TenantID: "tenant-a"})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for missing header, got %d", rec.Code)
	}

	req = httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok for matching sources, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for missing authenticated tenant source, got %d", rec.Code)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
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
