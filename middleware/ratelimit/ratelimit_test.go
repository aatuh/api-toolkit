package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
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

type fixedLimiter struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (l fixedLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return l.allowed, l.retryAfter, l.err
}
