package identity

import (
	"context"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestResolverClientIPTrustedProxy(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 203.0.113.10")

	ip, ok := resolver.ClientIP(req)
	if !ok {
		t.Fatal("expected client IP")
	}
	if ip.String() != "198.51.100.9" {
		t.Fatalf("expected forwarded IP, got %s", ip.String())
	}
}

func TestParseAddrValueRejectsOversizedInput(t *testing.T) {
	value := strings.Repeat(" ", maxAddrValueBytes+1) + "203.0.113.5"
	if _, ok := parseAddrValue(value); ok {
		t.Fatal("expected oversized address value to be rejected")
	}
}

func TestResolverClientIPUntrustedProxy(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")

	ip, ok := resolver.ClientIP(req)
	if !ok {
		t.Fatal("expected client IP")
	}
	if ip.String() != "203.0.113.10" {
		t.Fatalf("expected remote IP, got %s", ip.String())
	}
}

func TestResolverClientIPHostileXForwardedFor(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "unknown, 198.51.100.9")

	ip, ok := resolver.ClientIP(req)
	if !ok {
		t.Fatal("expected client IP")
	}
	if ip.String() != "203.0.113.10" {
		t.Fatalf("expected remote IP fallback, got %s", ip.String())
	}
}

func TestResolverClientIPForwardedChain(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("Forwarded", "for=198.51.100.9;proto=https, for=203.0.113.10")

	ip, ok := resolver.ClientIP(req)
	if !ok {
		t.Fatal("expected client IP")
	}
	if ip.String() != "198.51.100.9" {
		t.Fatalf("expected forwarded IP, got %s", ip.String())
	}
}

func TestResolverSchemeFromForwarded(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-Proto", "https")

	if scheme := resolver.Scheme(req); scheme != "https" {
		t.Fatalf("expected https, got %s", scheme)
	}
}

func TestResolverResolveHandlesNilRequest(t *testing.T) {
	got := (Resolver{}).Resolve(nil)
	if got.IP.IsValid() {
		t.Fatalf("expected invalid IP for nil request, got %s", got.IP)
	}
	if got.IPString != "" || got.Scheme != "" || got.Host != "" || got.RequestID != "" {
		t.Fatalf("unexpected nil request identity: %+v", got)
	}
}

func TestResolverResolveCollectsForwardedIdentity(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyBoth,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://internal.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("Forwarded", `for="[2001:db8::1]:443";proto=https;host=api.example.test`)
	req.Header.Set("X-Request-ID", " req-123 ")

	got := resolver.Resolve(req)
	if got.IPString != "2001:db8::1" {
		t.Fatalf("IPString = %q, want forwarded IPv6", got.IPString)
	}
	if got.Scheme != "https" {
		t.Fatalf("Scheme = %q, want https", got.Scheme)
	}
	if got.Host != "api.example.test" {
		t.Fatalf("Host = %q, want forwarded host", got.Host)
	}
	if got.RequestID != "req-123" {
		t.Fatalf("RequestID = %q, want trimmed request id", got.RequestID)
	}
}

func TestResolverIgnoresForwardedHostAndProtoFromUntrustedPeer(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyBoth,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://service.internal", nil)
	req.RemoteAddr = "198.51.100.44:4321"
	req.Header.Set("Forwarded", `for=192.0.2.9;proto=https;host=attacker.example`)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "attacker.example")

	if scheme := resolver.Scheme(req); scheme != "http" {
		t.Fatalf("Scheme = %q, want request scheme fallback", scheme)
	}
	if host := resolver.Host(req); host != "service.internal" {
		t.Fatalf("Host = %q, want request host fallback", host)
	}
}

func TestParseTrustedProxiesSupportsAddressesAndCIDRs(t *testing.T) {
	prefixes, err := ParseTrustedProxies([]string{" 203.0.113.10 ", "2001:db8::/32", ""})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}
	if !prefixes[0].Contains(netip.MustParseAddr("203.0.113.10")) {
		t.Fatalf("first prefix does not contain exact IPv4 address: %v", prefixes[0])
	}
	if !prefixes[1].Contains(netip.MustParseAddr("2001:db8::1")) {
		t.Fatalf("second prefix does not contain IPv6 test address: %v", prefixes[1])
	}
}

func TestParseTrustedProxiesRejectsInvalidInput(t *testing.T) {
	if _, err := ParseTrustedProxies([]string{"not an ip"}); err == nil {
		t.Fatal("expected invalid trusted proxy to fail")
	}
}

func TestRequestIDPrefersRequestIDOverCorrelationID(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.test", nil)
	req.Header.Set("X-Correlation-ID", "corr-123")
	req.Header.Set("X-Request-ID", " req-456 ")

	if got := RequestID(req); got != "req-456" {
		t.Fatalf("RequestID = %q, want X-Request-ID", got)
	}
}
