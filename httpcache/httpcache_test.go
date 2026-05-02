package httpcache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestETagConstructorsAndParser(t *testing.T) {
	if got := StrongETag("abc").String(); got != `"abc"` {
		t.Fatalf("StrongETag = %q", got)
	}
	if got := WeakETag("abc").String(); got != `W/"abc"` {
		t.Fatalf("WeakETag = %q", got)
	}
	if got := HashETag([]byte("abc")).String(); !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Fatalf("HashETag = %q, want quoted", got)
	}
	if _, err := ParseETag("abc"); err == nil {
		t.Fatal("expected malformed ETag error")
	}
	if got, err := ParseETag(`W/"abc"`); err != nil || !got.IsWeak() {
		t.Fatalf("ParseETag weak = %q, %v", got, err)
	}
}

func TestEvaluateReadUsesETagAndLastModified(t *testing.T) {
	validators := Validators{
		ETag:         StrongETag("v1"),
		LastModified: time.Date(2026, 5, 2, 8, 30, 15, 900, time.UTC),
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/1", nil)
	req.Header.Set("If-None-Match", `W/"v1"`)
	if decision := EvaluateRead(req, validators); !decision.NotModified || decision.Status != http.StatusNotModified {
		t.Fatalf("decision = %#v, want 304", decision)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/1", nil)
	req.Header.Set("If-Modified-Since", "Sat, 02 May 2026 08:30:15 GMT")
	if decision := EvaluateRead(req, validators); !decision.NotModified || decision.Status != http.StatusNotModified {
		t.Fatalf("decision = %#v, want 304", decision)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/1", nil)
	if decision := EvaluateRead(req, Validators{}); decision.Status != 0 {
		t.Fatalf("empty validators decision = %#v, want no-op", decision)
	}
}

func TestEvaluateWriteUsesStrongETagAndUnmodifiedSince(t *testing.T) {
	validators := Validators{
		ETag:         StrongETag("v1"),
		LastModified: time.Date(2026, 5, 2, 8, 30, 16, 0, time.UTC),
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/widgets/1", nil)
	req.Header.Set("If-Match", `W/"v1"`)
	if decision := EvaluateWrite(req, validators); !decision.PreconditionFailed || decision.Status != http.StatusPreconditionFailed {
		t.Fatalf("weak If-Match decision = %#v, want 412", decision)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/widgets/1", nil)
	req.Header.Set("If-Match", `"v1"`)
	if decision := EvaluateWrite(req, validators); decision.Status != 0 {
		t.Fatalf("strong If-Match decision = %#v, want no-op", decision)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/widgets/1", nil)
	req.Header.Set("If-Unmodified-Since", "Sat, 02 May 2026 08:30:15 GMT")
	if decision := EvaluateWrite(req, validators); !decision.PreconditionFailed || decision.Status != http.StatusPreconditionFailed {
		t.Fatalf("If-Unmodified-Since decision = %#v, want 412", decision)
	}
}

func TestWriters(t *testing.T) {
	validators := Validators{
		ETag:         StrongETag("v1"),
		LastModified: time.Date(2026, 5, 2, 8, 30, 15, 900, time.UTC),
	}
	rec := httptest.NewRecorder()
	WriteNotModified(rec, validators)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
	if rec.Header().Get("ETag") != `"v1"` {
		t.Fatalf("ETag = %q", rec.Header().Get("ETag"))
	}
	if rec.Header().Get("Last-Modified") != "Sat, 02 May 2026 08:30:15 GMT" {
		t.Fatalf("Last-Modified = %q", rec.Header().Get("Last-Modified"))
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	WritePreconditionFailed(rec)
	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":412`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
