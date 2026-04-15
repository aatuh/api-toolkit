package trace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

func TestTracePassThrough(t *testing.T) {
	incoming := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	state := "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"

	mw, err := New(Options{TrustIncoming: true})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	req.Header.Set(headerTraceParent, incoming)
	req.Header.Set(headerTraceState, state)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotTraceParent := rec.Header().Get(headerTraceParent)
	if gotTraceParent == "" {
		t.Fatal("expected traceparent response header")
	}
	if gotTraceID := rec.Header().Get(headerTraceID); gotTraceID == "" {
		t.Fatal("expected X-Trace-ID response header")
	}
	if gotState := rec.Header().Get(headerTraceState); gotState != state {
		t.Fatalf("tracestate = %q", gotState)
	}

	inTraceID, inSpanID, inFlags, ok := parseTraceParent(incoming)
	if !ok {
		t.Fatal("expected valid incoming traceparent")
	}
	gotTraceID, gotSpanID, gotFlags, ok := parseTraceParent(gotTraceParent)
	if !ok {
		t.Fatalf("expected valid traceparent, got %q", gotTraceParent)
	}
	if gotTraceID != inTraceID {
		t.Fatalf("trace_id = %q", gotTraceID)
	}
	if gotSpanID == inSpanID {
		t.Fatalf("expected new span id, got %q", gotSpanID)
	}
	if gotFlags != inFlags {
		t.Fatalf("trace_flags = %02x", gotFlags)
	}
}

func TestTraceGeneratesWhenUntrusted(t *testing.T) {
	incoming := "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-00"
	state := "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"

	mw, err := New(Options{TrustIncoming: false})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	req.Header.Set(headerTraceParent, incoming)
	req.Header.Set(headerTraceState, state)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotTraceParent := rec.Header().Get(headerTraceParent)
	if gotTraceParent == "" {
		t.Fatal("expected traceparent response header")
	}
	if gotTraceID := rec.Header().Get(headerTraceID); gotTraceID == "" {
		t.Fatal("expected X-Trace-ID response header")
	}
	if gotState := rec.Header().Get(headerTraceState); gotState != "" {
		t.Fatalf("expected tracestate to be empty, got %q", gotState)
	}

	inTraceID, _, _, ok := parseTraceParent(incoming)
	if !ok {
		t.Fatal("expected valid incoming traceparent")
	}
	gotTraceID, _, _, ok := parseTraceParent(gotTraceParent)
	if !ok {
		t.Fatalf("expected valid traceparent, got %q", gotTraceParent)
	}
	if gotTraceID == inTraceID {
		t.Fatalf("expected new trace id, got %q", gotTraceID)
	}
}

type staticIDGen struct {
	ids []string
	idx int
}

func (g *staticIDGen) New() string {
	if g == nil || g.idx >= len(g.ids) {
		return ""
	}
	id := g.ids[g.idx]
	g.idx++
	return id
}

func TestTraceUsesInjectedIDGen(t *testing.T) {
	traceGen := &staticIDGen{ids: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	spanGen := &staticIDGen{ids: []string{"bbbbbbbbbbbbbbbb"}}
	mw, err := New(Options{
		TraceIDGen: traceGen,
		SpanIDGen:  spanGen,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get(headerTraceParent)
	if got == "" {
		t.Fatal("expected traceparent response header")
	}
	if gotTraceID := rec.Header().Get(headerTraceID); gotTraceID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("X-Trace-ID = %q", gotTraceID)
	}
	gotTraceID, gotSpanID, _, ok := parseTraceParent(got)
	if !ok {
		t.Fatalf("expected valid traceparent, got %q", got)
	}
	if gotTraceID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("trace_id = %q", gotTraceID)
	}
	if gotSpanID != "bbbbbbbbbbbbbbbb" {
		t.Fatalf("span_id = %q", gotSpanID)
	}
}

func TestTraceEchoesRequestIDOnProblemResponses(t *testing.T) {
	traceGen := &staticIDGen{ids: []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	spanGen := &staticIDGen{ids: []string{"bbbbbbbbbbbbbbbb"}}
	mw, err := New(Options{
		TraceIDGen: traceGen,
		SpanIDGen:  spanGen,
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, httpx.ErrForbidden)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if got := rec.Header().Get(headerRequestID); got != "corr-123" {
		t.Fatalf("X-Request-ID = %q", got)
	}
	if got := rec.Header().Get(headerTraceID); got != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("X-Trace-ID = %q", got)
	}
	if got := rec.Header().Get(headerTraceParent); got == "" {
		t.Fatal("expected traceparent response header")
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q", got)
	}
}
