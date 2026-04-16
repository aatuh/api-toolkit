package recover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/ports"
)

type captureLogger struct {
	msg string
	kv  []any
}

func (l *captureLogger) Debug(string, ...any) {}
func (l *captureLogger) Info(string, ...any)  {}
func (l *captureLogger) Warn(string, ...any)  {}

func (l *captureLogger) Error(msg string, kv ...any) {
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func TestMiddlewareWritesProblemWhenNothingCommitted(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"detail":"internal server error"`) {
		t.Fatalf("expected internal error problem body, got %q", rec.Body.String())
	}
}

func TestMiddlewareAbortsAfterPartialWrite(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial:"))
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	defer assertAbortPanic(t, rec, http.StatusOK, "partial:")()
	handler.ServeHTTP(rec, req)
}

func TestMiddlewareAbortsAfterCommittedHeader(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	defer assertAbortPanic(t, rec, http.StatusNoContent, "")()
	handler.ServeHTTP(rec, req)
}

func TestNewLogsPanicsToProvidedLogger(t *testing.T) {
	log := &captureLogger{}
	handler := New(WithLogger(log))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if log.msg != "panic recovered" {
		t.Fatalf("unexpected log message: %q", log.msg)
	}
	fields := kvToMap(log.kv)
	if fields["panic"] != "boom" {
		t.Fatalf("panic field = %v", fields["panic"])
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatal("expected stack field")
	}
}

func TestNewCanDisableStackLogging(t *testing.T) {
	log := &captureLogger{}
	handler := New(WithLogger(log), WithStackLogging(false))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	if _, ok := fields["stack"]; ok {
		t.Fatal("did not expect stack field")
	}
}

func TestMiddlewareRemainsCompatibleWithoutLogger(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func kvToMap(kv []any) map[string]any {
	out := make(map[string]any)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = kv[i+1]
	}
	return out
}

var _ ports.Logger = (*captureLogger)(nil)

func assertAbortPanic(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantBody string) func() {
	t.Helper()
	return func() {
		got := recover()
		if got != http.ErrAbortHandler {
			t.Fatalf("expected panic %v, got %v", http.ErrAbortHandler, got)
		}
		if rec.Code != wantCode {
			t.Fatalf("expected committed status %d in recorder, got %d", wantCode, rec.Code)
		}
		if rec.Body.String() != wantBody {
			t.Fatalf("expected recorder body %q, got %q", wantBody, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "internal server error") {
			t.Fatalf("expected no appended problem body, got %q", rec.Body.String())
		}
	}
}
