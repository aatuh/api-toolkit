package identity

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestResolverClientIPTrustedProxy(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequest("GET", "http://example.test", nil)
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

func TestResolverClientIPUntrustedProxy(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
	}
	req := httptest.NewRequest("GET", "http://example.test", nil)
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

func TestResolverSchemeFromForwarded(t *testing.T) {
	resolver := Resolver{
		HeaderPolicy: HeaderPolicyXForwarded,
		TrustedProxies: []netip.Prefix{
			netip.MustParsePrefix("203.0.113.0/24"),
		},
	}
	req := httptest.NewRequest("GET", "http://example.test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-Proto", "https")

	if scheme := resolver.Scheme(req); scheme != "https" {
		t.Fatalf("expected https, got %s", scheme)
	}
}
