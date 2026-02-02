package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// Resolver resolves hostnames for SSRF protection.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// SSRFOptions configures SSRF protection for outbound HTTP.
type SSRFOptions struct {
	Transport    *http.Transport
	Resolver     Resolver
	Dialer       *net.Dialer
	AllowedHosts []string
	AllowedPorts []int
	AllowedCIDRs []string
	// AllowRedirectSchemeChange permits http<->https redirects.
	AllowRedirectSchemeChange bool
}

// SSRFTransport validates outbound targets before dialing and on redirects.
//
// Threat model coverage:
// - Blocks private/reserved IPv4/IPv6 ranges (including link-local/metadata IPs).
// - Re-validates each redirect hop (host/port/scheme) via CheckRedirect.
// - Mitigates DNS rebinding by resolving once and dialing by IP.
//
// Non-goals:
// - If you allowlist a host/CIDR, requests to that target are allowed.
// - It does not inspect application-level SSRF beyond host/port/scheme/IP.
//
// Example:
//
//	guard, _ := httpclient.NewSSRFTransport(httpclient.SSRFOptions{
//		AllowedHosts: []string{"api.example.com"},
//		AllowedPorts: []int{443},
//	})
//	client := &http.Client{
//		Transport:     guard,
//		CheckRedirect: guard.CheckRedirect,
//	}
type SSRFTransport struct {
	base                      *http.Transport
	guard                     *ssrfGuard
	allowRedirectSchemeChange bool
}

type ssrfGuard struct {
	resolver     Resolver
	dialer       *net.Dialer
	allowedHosts []string
	allowedPorts map[int]struct{}
	allowedCIDRs []netip.Prefix
}

type resolvedAddr struct {
	addr string
	ips  []netip.Addr
}

type resolvedAddrKey struct{}

// NewSSRFTransport creates a transport that blocks private/reserved networks by default.
func NewSSRFTransport(opts SSRFOptions) (*SSRFTransport, error) {
	guard, err := newSSRFGuard(opts)
	if err != nil {
		return nil, err
	}
	base := cloneTransport(opts.Transport)
	base.DialContext = guard.wrapDial(base.DialContext)
	if base.DialTLSContext != nil {
		base.DialTLSContext = guard.wrapDial(base.DialTLSContext)
	}
	return &SSRFTransport{
		base:                      base,
		guard:                     guard,
		allowRedirectSchemeChange: opts.AllowRedirectSchemeChange,
	}, nil
}

// RoundTrip validates the outbound request before dialing.
func (t *SSRFTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.base == nil || t.guard == nil {
		return nil, errors.New("ssrf transport not configured")
	}
	if req == nil {
		return nil, errors.New("request is nil")
	}
	ctx := req.Context()
	resolved, err := t.guard.validateRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		ctx = context.WithValue(ctx, resolvedAddrKey{}, *resolved)
	}
	next := req.Clone(ctx)
	return t.base.RoundTrip(next)
}

// CheckRedirect validates redirect targets and enforces scheme-change rules.
func (t *SSRFTransport) CheckRedirect(req *http.Request, via []*http.Request) error {
	if t == nil || t.guard == nil {
		return errors.New("ssrf transport not configured")
	}
	if req == nil || req.URL == nil {
		return errors.New("redirect request url is nil")
	}
	if len(via) > 0 && !t.allowRedirectSchemeChange {
		prev := via[len(via)-1]
		if prev != nil && prev.URL != nil {
			prevScheme := strings.ToLower(strings.TrimSpace(prev.URL.Scheme))
			nextScheme := strings.ToLower(strings.TrimSpace(req.URL.Scheme))
			if prevScheme != "" && nextScheme != "" && prevScheme != nextScheme {
				return fmt.Errorf("redirect scheme change %q -> %q not allowed", prevScheme, nextScheme)
			}
		}
	}
	if _, err := t.guard.validateRequest(req.Context(), req); err != nil {
		return err
	}
	return nil
}

func newSSRFGuard(opts SSRFOptions) (*ssrfGuard, error) {
	allowedCIDRs, err := parseCIDRs(opts.AllowedCIDRs)
	if err != nil {
		return nil, err
	}
	allowedPorts, err := normalizePorts(opts.AllowedPorts)
	if err != nil {
		return nil, err
	}
	allowedHosts := normalizeHosts(opts.AllowedHosts)
	resolver := opts.Resolver
	if resolver == nil {
		resolver = stdResolver{resolver: net.DefaultResolver}
	}
	dialer := opts.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	return &ssrfGuard{
		resolver:     resolver,
		dialer:       dialer,
		allowedHosts: allowedHosts,
		allowedPorts: allowedPorts,
		allowedCIDRs: allowedCIDRs,
	}, nil
}

func (g *ssrfGuard) validateRequest(ctx context.Context, req *http.Request) (*resolvedAddr, error) {
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	if req == nil || req.URL == nil {
		return nil, errors.New("request url is nil")
	}
	scheme := strings.ToLower(strings.TrimSpace(req.URL.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if host == "" {
		return nil, errors.New("host is required")
	}
	port, err := portForURL(req.URL)
	if err != nil {
		return nil, err
	}
	if !hostAllowed(host, g.allowedHosts) {
		return nil, fmt.Errorf("host %q not in allowlist", host)
	}
	if !portAllowed(port, g.allowedPorts) {
		return nil, fmt.Errorf("port %d not in allowlist", port)
	}
	ips, err := g.resolveHost(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := g.validateAddrs(ips); err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return &resolvedAddr{addr: addr, ips: ips}, nil
}

func (g *ssrfGuard) resolveHost(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	addrs, err := g.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}
	out := make([]netip.Addr, 0, len(addrs))
	seen := make(map[netip.Addr]struct{}, len(addrs))
	for _, addr := range addrs {
		ip, ok := netip.AddrFromSlice(addr.IP)
		if !ok {
			continue
		}
		ip = ip.Unmap()
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, errors.New("no ip addresses resolved")
	}
	return out, nil
}

func (g *ssrfGuard) validateAddrs(addrs []netip.Addr) error {
	for _, ip := range addrs {
		if !g.ipAllowed(ip) {
			return fmt.Errorf("ip %s is not allowed", ip.String())
		}
	}
	return nil
}

func (g *ssrfGuard) ipAllowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	if len(g.allowedCIDRs) > 0 {
		return ipInPrefixes(ip, g.allowedCIDRs)
	}
	return !isReservedIP(ip)
}

func (g *ssrfGuard) wrapDial(base func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	if base == nil {
		base = g.dialer.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		if ctx == nil {
			return nil, errors.New("context is nil")
		}
		host, port, err := splitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips := g.ipsForAddr(ctx, addr)
		if len(ips) == 0 {
			ips, err = g.resolveHost(ctx, host)
			if err != nil {
				return nil, err
			}
			if err := g.validateAddrs(ips); err != nil {
				return nil, err
			}
		}
		target := net.JoinHostPort(ips[0].String(), strconv.Itoa(port))
		return base(ctx, network, target)
	}
}

func (g *ssrfGuard) ipsForAddr(ctx context.Context, addr string) []netip.Addr {
	if ctx == nil {
		return nil
	}
	val := ctx.Value(resolvedAddrKey{})
	if val == nil {
		return nil
	}
	resolved, ok := val.(resolvedAddr)
	if !ok || resolved.addr != addr {
		return nil
	}
	return resolved.ips
}

func cloneTransport(base *http.Transport) *http.Transport {
	if base == nil {
		if def, ok := http.DefaultTransport.(*http.Transport); ok {
			return def.Clone()
		}
		return (&http.Transport{}).Clone()
	}
	return base.Clone()
}

func portForURL(u *url.URL) (int, error) {
	if u == nil {
		return 0, errors.New("url is nil")
	}
	if port := strings.TrimSpace(u.Port()); port != "" {
		val, err := strconv.Atoi(port)
		if err != nil || val <= 0 || val > 65535 {
			return 0, fmt.Errorf("invalid port %q", port)
		}
		return val, nil
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return 443, nil
	case "http":
		return 80, nil
	default:
		return 0, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port %q", portStr)
	}
	return host, port, nil
}

func normalizeHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	for _, raw := range hosts {
		val := strings.ToLower(strings.TrimSpace(raw))
		val = strings.TrimSuffix(val, ".")
		if val == "" {
			continue
		}
		out = append(out, val)
	}
	return out
}

func hostAllowed(host string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	for _, pattern := range allowlist {
		if pattern == "*" {
			return true
		}
		if host == pattern {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
		if strings.HasPrefix(pattern, ".") {
			if strings.HasSuffix(host, pattern) {
				return true
			}
		}
	}
	return false
}

func normalizePorts(ports []int) (map[int]struct{}, error) {
	out := make(map[int]struct{}, len(ports))
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid port %d", port)
		}
		out[port] = struct{}{}
	}
	return out, nil
}

func portAllowed(port int, allowlist map[int]struct{}) bool {
	if len(allowlist) == 0 {
		return true
	}
	_, ok := allowlist[port]
	return ok
}

func parseCIDRs(raw []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(raw))
	for _, entry := range raw {
		val := strings.TrimSpace(entry)
		if val == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(val); err == nil {
			out = append(out, prefix)
			continue
		}
		if addr, err := netip.ParseAddr(val); err == nil {
			out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		return nil, fmt.Errorf("invalid cidr %q", val)
	}
	return out, nil
}

func ipInPrefixes(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func isReservedIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip.Is4() {
		return ipInPrefixes(ip, blockedIPv4CIDRs)
	}
	return ipInPrefixes(ip, blockedIPv6CIDRs)
}

type stdResolver struct {
	resolver *net.Resolver
}

func (s stdResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if s.resolver == nil {
		return nil, errors.New("resolver is nil")
	}
	return s.resolver.LookupIPAddr(ctx, host)
}

func mustPrefix(raw string) netip.Prefix {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(err)
	}
	return prefix
}

var blockedIPv4CIDRs = []netip.Prefix{
	mustPrefix("0.0.0.0/8"),
	mustPrefix("10.0.0.0/8"),
	mustPrefix("100.64.0.0/10"),
	mustPrefix("127.0.0.0/8"),
	mustPrefix("169.254.0.0/16"),
	mustPrefix("172.16.0.0/12"),
	mustPrefix("192.0.0.0/24"),
	mustPrefix("192.0.2.0/24"),
	mustPrefix("192.88.99.0/24"),
	mustPrefix("192.168.0.0/16"),
	mustPrefix("198.18.0.0/15"),
	mustPrefix("198.51.100.0/24"),
	mustPrefix("203.0.113.0/24"),
	mustPrefix("224.0.0.0/4"),
	mustPrefix("240.0.0.0/4"),
	mustPrefix("255.255.255.255/32"),
}

var blockedIPv6CIDRs = []netip.Prefix{
	mustPrefix("::/128"),
	mustPrefix("::1/128"),
	mustPrefix("64:ff9b::/96"),
	mustPrefix("100::/64"),
	mustPrefix("2001:db8::/32"),
	mustPrefix("2001:10::/28"),
	mustPrefix("fc00::/7"),
	mustPrefix("fe80::/10"),
	mustPrefix("ff00::/8"),
}
