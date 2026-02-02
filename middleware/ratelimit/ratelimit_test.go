package ratelimit

import (
	"net/netip"
	"testing"

	"github.com/aatuh/api-toolkit/httpx/identity"
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
