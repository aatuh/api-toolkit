package recover

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/ports"
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

var errRecoveryResponseWrite = errors.New("response write failed")

type failingRecoveryResponseWriter struct {
	header     http.Header
	status     int
	writeCalls int
}

func (w *failingRecoveryResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingRecoveryResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *failingRecoveryResponseWriter) Write([]byte) (int, error) {
	w.writeCalls++
	return 0, errRecoveryResponseWrite
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

func TestMiddlewareStopsWhenRecoveryProblemWriteFails(t *testing.T) {
	log := &captureLogger{}
	handler := New(WithLogger(log), WithStackLogging(false))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	writer := &failingRecoveryResponseWriter{}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(writer, req)

	if writer.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", writer.status, http.StatusInternalServerError)
	}
	if writer.writeCalls != 1 {
		t.Fatalf("write calls = %d, want 1", writer.writeCalls)
	}
	if log.msg != "panic recovered" {
		t.Fatalf("log message = %q, want panic recovered", log.msg)
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

func TestMiddlewareAbortsAfterFlushCommit(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	defer assertAbortPanic(t, rec, http.StatusOK, "")()
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

func TestNewWithNilLoggerUsesNoopLogger(t *testing.T) {
	handler := New(WithLogger(nil))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestMiddlewareReraisesAbortHandler(t *testing.T) {
	handler := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	defer func() {
		got := recover()
		err, ok := got.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected abort handler panic, got %v", got)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("expected no recovery body for abort handler, got %q", rec.Body.String())
		}
	}()
	handler.ServeHTTP(rec, req)
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
		err, ok := got.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
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
