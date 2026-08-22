package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
)

func TestNewDefaultsClock(t *testing.T) {
	mw, err := New(Options{})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.opts.Clock == nil {
		t.Fatal("expected default clock")
	}
}

func TestNewRejectsDangerousBypassWithoutConfig(t *testing.T) {
	if _, err := New(Options{AllowDangerousDevBypasses: true}); err == nil {
		t.Fatal("expected error for missing skip header")
	}
	if _, err := New(Options{
		AllowDangerousDevBypasses: true,
		SkipHeader:                "X-RateLimit-Skip",
	}); err == nil {
		t.Fatal("expected error for missing trusted proxies")
	}
	_, err := New(Options{
		AllowDangerousDevBypasses: true,
		SkipHeader:                "X-RateLimit-Skip",
		ClientIPResolver: identity.Resolver{
			TrustedProxies: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRejectsBothLimiterContracts(t *testing.T) {
	_, err := New(Options{
		Limiter:         fixedLimiter{allowed: true},
		DecisionLimiter: fixedDecisionLimiter{decision: Decision{Allowed: true}},
	})
	if err == nil {
		t.Fatal("expected error when both limiter contracts are configured")
	}
}

func TestHandlerUsesSharedAnonymousBucketForBlankKeys(t *testing.T) {
	mw, err := New(Options{
		Capacity:   1,
		RefillRate: 1,
		Clock:      headerClock{now: time.Unix(1_000, 0).UTC()},
		Key:        func(*http.Request) string { return " \t" },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for request := 1; request <= 2; request++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
		want := http.StatusNoContent
		if request == 2 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("request %d status = %d, want %d", request, recorder.Code, want)
		}
	}
}

func TestHandlerRejectsDangerousBypassFromUntrustedPeer(t *testing.T) {
	mw, err := New(Options{
		Capacity:                  1,
		RefillRate:                1,
		Clock:                     headerClock{now: time.Unix(1_000, 0).UTC()},
		Key:                       func(*http.Request) string { return "client-1" },
		SkipEnabled:               true,
		SkipHeader:                "X-RateLimit-Skip",
		AllowDangerousDevBypasses: true,
		ClientIPResolver: identity.Resolver{
			TrustedProxies: []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for request := 1; request <= 2; request++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.RemoteAddr = "198.51.100.1:443"
		req.Header.Set("X-RateLimit-Skip", "true")
		handler.ServeHTTP(recorder, req)
		want := http.StatusNoContent
		if request == 2 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("request %d status = %d, want %d", request, recorder.Code, want)
		}
	}
}

func TestHandlerRoundsSharedLimiterRetryAfterUp(t *testing.T) {
	mw, err := New(Options{
		Limiter: fixedLimiter{
			allowed:    false,
			retryAfter: 250 * time.Millisecond,
		},
		Key: func(*http.Request) string { return "client-1" },
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expected request to be rate limited")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("expected Retry-After to round up to 1, got %q", got)
	}
}

func TestMiddlewareReportsCheckedProblemWriteFailure(t *testing.T) {
	want := errors.New("response body write failed")
	var got error
	mw, err := New(Options{
		OnError: func(err error) {
			got = err
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	mw.writeProblem(&failingResponseWriter{err: want}, http.StatusTooManyRequests, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeRateLimited),
		Title:  http.StatusText(http.StatusTooManyRequests),
		Detail: "rate limit exceeded",
	})

	if !errors.Is(got, want) {
		t.Fatalf("OnError = %v, want response write error wrapping %v", got, want)
	}
	var responseErr *httpx.ResponseWriteError
	if !errors.As(got, &responseErr) || responseErr.Stage != httpx.ResponseWriteStageBody {
		t.Fatalf("OnError = %T %v, want body-stage ResponseWriteError", got, got)
	}
}

type failingResponseWriter struct {
	header http.Header
	err    error
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (*failingResponseWriter) WriteHeader(int) {}

func (w *failingResponseWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type fixedLimiter struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (l fixedLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return l.allowed, l.retryAfter, l.err
}
