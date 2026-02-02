package trace

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set(headerTraceParent, incoming)
	req.Header.Set(headerTraceState, state)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotTraceParent := rec.Header().Get(headerTraceParent)
	if gotTraceParent == "" {
		t.Fatal("expected traceparent response header")
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

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set(headerTraceParent, incoming)
	req.Header.Set(headerTraceState, state)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotTraceParent := rec.Header().Get(headerTraceParent)
	if gotTraceParent == "" {
		t.Fatal("expected traceparent response header")
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

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get(headerTraceParent)
	if got == "" {
		t.Fatal("expected traceparent response header")
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
