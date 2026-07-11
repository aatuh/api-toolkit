package clerk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/healthchecktest"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestSubjectContextAndDisabledMiddleware(t *testing.T) {
	ctx := WithSubject(context.Background(), Subject{UserID: "user_1", Email: "user@example.com"})
	subject, ok := SubjectFromContext(ctx)
	if !ok || subject.UserID != "user_1" || subject.Email != "user@example.com" {
		t.Fatalf("SubjectFromContext() = %#v, %v", subject, ok)
	}
	if _, ok := SubjectFromContext(context.Background()); ok {
		t.Fatal("expected empty context to have no subject")
	}

	mw := &Middleware{enabled: false, log: ports.NopLogger{}}
	called := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("disabled handler called=%v status=%d", called, rec.Code)
	}

	called = false
	handler = mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusAccepted {
		t.Fatalf("disabled optional handler called=%v status=%d", called, rec.Code)
	}
}

func TestNewMiddlewareDisabledAndHealthCheckerDisabled(t *testing.T) {
	mw, err := NewMiddleware(context.Background(), Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	mw.Close()
	mw.Close()
	if HealthChecker(Config{Enabled: false, JWKSURL: "http://example.com/jwks"}, nil) != nil {
		t.Fatal("disabled health checker should be nil")
	}
	if HealthChecker(Config{Enabled: true}, nil) != nil {
		t.Fatal("missing JWKS URL health checker should be nil")
	}
	var nilMiddleware *Middleware
	if nilMiddleware.Handler(http.NotFoundHandler()) == nil || nilMiddleware.OptionalHandler(http.NotFoundHandler()) == nil {
		t.Fatal("nil middleware should pass handlers through")
	}
	nilMiddleware.Close()
}

func TestHealthCheckerContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := HealthChecker(Config{Enabled: true, JWKSURL: server.URL, JWKSRefreshTimeout: time.Second}, server.Client())
	healthchecktest.AssertCheckerContract(t, checker, "clerk", health.StatusHealthy)
}
