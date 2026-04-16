package requestlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
)

type captureLogger struct {
	level string
	msg   string
	kv    []any
}

func (l *captureLogger) Debug(string, ...any) {}
func (l *captureLogger) Warn(msg string, kv ...any) {
	l.level = "warn"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (l *captureLogger) Error(msg string, kv ...any) {
	l.level = "error"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (l *captureLogger) Info(msg string, kv ...any) {
	l.level = "info"
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func TestRequestLogRedactionDefaults(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log,
		WithRoutePattern(func(*http.Request) string { return "/demo" }),
		WithRequestHeaders(),
		WithResponseHeaders(),
	)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "sid=secret")
		w.Header().Set("X-Resp", "ok")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/demo", nil)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "a=b")
	req.Header.Set("X-API-Key", "abc123")
	req.Header.Set("X-Extra", "ok")
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	reqHeaders, ok := fields[FieldRequestHeaders].(map[string]string)
	if !ok {
		t.Fatalf("expected request headers in log")
	}
	if reqHeaders["Authorization"] != redactedValue {
		t.Fatalf("authorization not redacted: %q", reqHeaders["Authorization"])
	}
	if reqHeaders["Cookie"] != redactedValue {
		t.Fatalf("cookie not redacted: %q", reqHeaders["Cookie"])
	}
	if reqHeaders["X-Api-Key"] != redactedValue {
		t.Fatalf("api key not redacted: %q", reqHeaders["X-Api-Key"])
	}
	if reqHeaders["X-Extra"] != "ok" {
		t.Fatalf("unexpected extra header: %q", reqHeaders["X-Extra"])
	}

	respHeaders, ok := fields[FieldResponseHeaders].(map[string]string)
	if !ok {
		t.Fatalf("expected response headers in log")
	}
	if respHeaders["Set-Cookie"] != redactedValue {
		t.Fatalf("set-cookie not redacted: %q", respHeaders["Set-Cookie"])
	}
	if respHeaders["X-Resp"] != "ok" {
		t.Fatalf("unexpected response header: %q", respHeaders["X-Resp"])
	}
}

func TestRequestLogFieldConventions(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/test" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	traceID := oteltrace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := oteltrace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)
	req.Header.Set("X-Request-ID", "req-xyz")
	req = req.WithContext(oteltrace.ContextWithSpanContext(req.Context(), sc))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	if fields[FieldRequestID] != "req-xyz" {
		t.Fatalf("request_id = %v", fields[FieldRequestID])
	}
	if fields[FieldTraceID] != traceID.String() {
		t.Fatalf("trace_id = %v", fields[FieldTraceID])
	}
	if fields[FieldSpanID] != spanID.String() {
		t.Fatalf("span_id = %v", fields[FieldSpanID])
	}
	if fields[FieldRoute] != "/test" {
		t.Fatalf("route = %v", fields[FieldRoute])
	}
	if fields[FieldStatus] != http.StatusOK {
		t.Fatalf("status = %v", fields[FieldStatus])
	}
	if _, ok := fields[FieldLatencyMS]; !ok {
		t.Fatalf("missing latency field")
	}
}

func TestRequestLogDefaultsClock(t *testing.T) {
	mw, err := New(nil)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.clock == nil {
		t.Fatal("expected default clock")
	}
}

func TestRequestLogDoesNotAttachStackForHandled5xxByDefault(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/boom" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if log.level != "error" {
		t.Fatalf("level = %q", log.level)
	}
	fields := kvToMap(log.kv)
	if _, ok := fields[FieldStack]; ok {
		t.Fatal("did not expect stack field")
	}
}

func TestRequestLogCanAttachStackForHandled5xxWhenEnabled(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log,
		WithRoutePattern(func(*http.Request) string { return "/boom" }),
		With5xxStackLogging(true),
	)
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	fields := kvToMap(log.kv)
	if _, ok := fields[FieldStack]; !ok {
		t.Fatal("expected stack field")
	}
}

func TestRequestLogEmitsErrorForRecoveredPanicBeforeCommit(t *testing.T) {
	log := &captureLogger{}
	mw, err := New(log, WithRoutePattern(func(*http.Request) string { return "/panic" }))
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/panic", nil)
	defer func() {
		if got := recover(); got != "boom" {
			t.Fatalf("expected panic boom, got %v", got)
		}
		if log.level != "error" {
			t.Fatalf("level = %q", log.level)
		}
		fields := kvToMap(log.kv)
		if fields[FieldStatus] != http.StatusInternalServerError {
			t.Fatalf("status = %v", fields[FieldStatus])
		}
		if fields[FieldPanicRecovered] != true {
			t.Fatalf("panic_recovered = %v", fields[FieldPanicRecovered])
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
