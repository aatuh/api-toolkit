package tenant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v4/authorization"
)

var errTenantResponseWrite = errors.New("response write failed")

type failingTenantResponseWriter struct {
	header     http.Header
	status     int
	writeCalls int
}

func (w *failingTenantResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingTenantResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingTenantResponseWriter) Write([]byte) (int, error) {
	w.writeCalls++
	return 0, errTenantResponseWrite
}

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

func TestTenantMiddlewareStopsAfterProblemWriteFailure(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		request func() *http.Request
		status  int
	}{
		{
			name:    "missing tenant for unauthenticated request",
			options: Options{HeaderName: "X-Tenant-ID"},
			request: func() *http.Request {
				return httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			},
			status: http.StatusUnauthorized,
		},
		{
			name:    "missing tenant for authenticated request",
			options: Options{HeaderName: "X-Tenant-ID"},
			request: func() *http.Request {
				req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
				return req.WithContext(authorization.WithActor(req.Context(), authorization.Actor{UserID: "user-1"}))
			},
			status: http.StatusForbidden,
		},
		{
			name: "mismatched tenant sources",
			options: Options{
				HeaderName: "X-Tenant-ID",
				TenantFromContext: func(context.Context) (string, bool) {
					return "tenant-a", true
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
				req.Header.Set("X-Tenant-ID", "tenant-b")
				return req
			},
			status: http.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mw, err := New(test.options)
			if err != nil {
				t.Fatalf("new middleware: %v", err)
			}

			writer := &failingTenantResponseWriter{}
			nextCalled := false
			mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				nextCalled = true
			})).ServeHTTP(writer, test.request())

			if writer.status != test.status {
				t.Fatalf("status = %d, want %d", writer.status, test.status)
			}
			if writer.writeCalls != 1 {
				t.Fatalf("write calls = %d, want 1", writer.writeCalls)
			}
			if nextCalled {
				t.Fatal("next handler was called after tenant failure")
			}
		})
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
