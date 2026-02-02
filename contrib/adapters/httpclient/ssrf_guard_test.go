package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"
)

type stubResolver struct {
	addrs []netip.Addr
	err   error
}

func (s stubResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]net.IPAddr, 0, len(s.addrs))
	for _, addr := range s.addrs {
		out = append(out, net.IPAddr{IP: net.ParseIP(addr.String())})
	}
	return out, nil
}

type countingResolver struct {
	responses [][]netip.Addr
	calls     int
}

func (c *countingResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	idx := c.calls
	c.calls++
	if idx >= len(c.responses) {
		return nil, errors.New("unexpected resolver call")
	}
	addrs := c.responses[idx]
	out := make([]net.IPAddr, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, net.IPAddr{IP: net.ParseIP(addr.String())})
	}
	return out, nil
}

func TestSSRFGuardBlocksPrivateIPv4(t *testing.T) {
	guard, err := newSSRFGuard(SSRFOptions{})
	if err != nil {
		t.Fatalf("new guard: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://10.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := guard.validateRequest(context.Background(), req); err == nil {
		t.Fatal("expected private ip to be blocked")
	}
}

func TestSSRFGuardBlocksIPv6Loopback(t *testing.T) {
	guard, err := newSSRFGuard(SSRFOptions{})
	if err != nil {
		t.Fatalf("new guard: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://[::1]/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := guard.validateRequest(context.Background(), req); err == nil {
		t.Fatal("expected loopback ip to be blocked")
	}
}

func TestSSRFGuardBlocksRebinding(t *testing.T) {
	guard, err := newSSRFGuard(SSRFOptions{
		Resolver: stubResolver{
			addrs: []netip.Addr{
				netip.MustParseAddr("93.184.216.34"),
				netip.MustParseAddr("127.0.0.1"),
			},
		},
	})
	if err != nil {
		t.Fatalf("new guard: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := guard.validateRequest(context.Background(), req); err == nil {
		t.Fatal("expected rebinding to be blocked")
	}
}

func TestSSRFGuardAllowsCIDRAllowlist(t *testing.T) {
	guard, err := newSSRFGuard(SSRFOptions{
		AllowedCIDRs: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("new guard: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://10.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := guard.validateRequest(context.Background(), req); err != nil {
		t.Fatalf("expected allowlist to permit private ip: %v", err)
	}
}

func TestSSRFGuardBlocksRedirectSchemeChange(t *testing.T) {
	guard, err := NewSSRFTransport(SSRFOptions{
		AllowedCIDRs: []string{"127.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("new transport: %v", err)
	}
	prev, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	next, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := guard.CheckRedirect(next, []*http.Request{prev}); err == nil {
		t.Fatal("expected redirect scheme change to be blocked")
	}
}

func TestSSRFGuardAllowsRedirectSchemeChangeWhenEnabled(t *testing.T) {
	guard, err := NewSSRFTransport(SSRFOptions{
		AllowedCIDRs:              []string{"127.0.0.0/8"},
		AllowRedirectSchemeChange: true,
	})
	if err != nil {
		t.Fatalf("new transport: %v", err)
	}
	prev, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	next, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if err := guard.CheckRedirect(next, []*http.Request{prev}); err != nil {
		t.Fatalf("expected redirect scheme change to be allowed: %v", err)
	}
}

func TestSSRFGuardRedirectChainRevalidated(t *testing.T) {
	statuses := []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	}
	for _, status := range statuses {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", status)
			}))
			defer second.Close()
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, second.URL, status)
			}))
			defer first.Close()

			guard, err := NewSSRFTransport(SSRFOptions{
				AllowedCIDRs: []string{"127.0.0.0/8"},
			})
			if err != nil {
				t.Fatalf("new transport: %v", err)
			}
			client := &http.Client{
				Transport:     guard,
				CheckRedirect: guard.CheckRedirect,
			}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, first.URL, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := client.Do(req)
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if err == nil {
				t.Fatal("expected redirect to blocked host")
			}
		})
	}
}

func TestSSRFGuardDialUsesResolvedIP(t *testing.T) {
	resolver := &countingResolver{
		responses: [][]netip.Addr{
			{netip.MustParseAddr("93.184.216.34")},
			{netip.MustParseAddr("127.0.0.1")},
		},
	}
	guard, err := newSSRFGuard(SSRFOptions{Resolver: resolver})
	if err != nil {
		t.Fatalf("new guard: %v", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/resource", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resolved, err := guard.validateRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	ctx := context.WithValue(context.Background(), resolvedAddrKey{}, *resolved)

	var dialed string
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	dial := guard.wrapDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = addr
		return clientConn, nil
	})
	conn, err := dial(ctx, "tcp", "example.test:80")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()

	if resolver.calls != 1 {
		t.Fatalf("expected 1 resolve call, got %d", resolver.calls)
	}
	if dialed != "93.184.216.34:80" {
		t.Fatalf("expected dial to use resolved ip, got %s", dialed)
	}
}
