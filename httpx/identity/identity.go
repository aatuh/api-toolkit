package identity

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// HeaderPolicy controls which forwarded headers may be honored.
type HeaderPolicy uint8

const (
	// HeaderPolicyNone ignores forwarded headers.
	HeaderPolicyNone HeaderPolicy = 0
	// HeaderPolicyXForwarded trusts X-Forwarded-* headers from trusted proxies.
	HeaderPolicyXForwarded HeaderPolicy = 1 << iota
	// HeaderPolicyForwarded trusts RFC 7239 Forwarded headers from trusted proxies.
	HeaderPolicyForwarded
	// HeaderPolicyBoth trusts both Forwarded and X-Forwarded-* headers.
	HeaderPolicyBoth = HeaderPolicyXForwarded | HeaderPolicyForwarded
)

// Resolver derives canonical client identity values from an http.Request.
// Forwarded headers are honored only when the direct peer is trusted.
type Resolver struct {
	TrustedProxies []netip.Prefix
	HeaderPolicy   HeaderPolicy
}

// ClientInfo captures canonical client identity attributes.
type ClientInfo struct {
	IP        netip.Addr
	IPString  string
	Scheme    string
	Host      string
	RequestID string
}

// Resolve extracts the canonical client identity from the request.
func (r Resolver) Resolve(req *http.Request) ClientInfo {
	ip, _ := r.ClientIP(req)
	ipStr := ""
	if ip.IsValid() {
		ipStr = ip.String()
	}
	return ClientInfo{
		IP:        ip,
		IPString:  ipStr,
		Scheme:    r.Scheme(req),
		Host:      r.Host(req),
		RequestID: RequestID(req),
	}
}

// ClientIP returns the best-effort client IP address.
func (r Resolver) ClientIP(req *http.Request) (netip.Addr, bool) {
	if req == nil {
		return netip.Addr{}, false
	}
	remote, remoteOK := parseRemoteAddr(req.RemoteAddr)
	if r.trustsRemote(remote) {
		if r.usesForwarded() {
			if addr, ok := forwardedFor(req.Header.Get("Forwarded")); ok {
				return addr, true
			}
		}
		if r.usesXForwarded() {
			if addr, ok := xForwardedFor(req.Header.Get("X-Forwarded-For")); ok {
				return addr, true
			}
		}
	}
	if remoteOK {
		return remote, true
	}
	return netip.Addr{}, false
}

// ClientIPString returns the best-effort client IP string.
func (r Resolver) ClientIPString(req *http.Request) string {
	if ip, ok := r.ClientIP(req); ok {
		return ip.String()
	}
	return ""
}

// Scheme returns the request scheme, honoring forwarded headers only for trusted proxies.
func (r Resolver) Scheme(req *http.Request) string {
	if req == nil {
		return ""
	}
	if r.trustsRemoteAddr(req.RemoteAddr) {
		if r.usesForwarded() {
			if proto := forwardedProto(req.Header.Get("Forwarded")); proto != "" {
				return proto
			}
		}
		if r.usesXForwarded() {
			if proto := xForwardedProto(req.Header.Get("X-Forwarded-Proto")); proto != "" {
				return proto
			}
		}
	}
	if req.URL != nil && req.URL.Scheme != "" {
		return req.URL.Scheme
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

// Host returns the request host, honoring forwarded headers only for trusted proxies.
func (r Resolver) Host(req *http.Request) string {
	if req == nil {
		return ""
	}
	if r.trustsRemoteAddr(req.RemoteAddr) {
		if r.usesForwarded() {
			if host := forwardedHost(req.Header.Get("Forwarded")); host != "" {
				return host
			}
		}
		if r.usesXForwarded() {
			if host := xForwardedHost(req.Header.Get("X-Forwarded-Host")); host != "" {
				return host
			}
		}
	}
	return req.Host
}

// RequestID extracts a request ID from standard headers.
func RequestID(req *http.Request) string {
	if req == nil {
		return ""
	}
	if v := strings.TrimSpace(req.Header.Get("X-Request-ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(req.Header.Get("X-Correlation-ID")); v != "" {
		return v
	}
	return ""
}

// ParseTrustedProxies parses CIDR strings into prefixes.
func ParseTrustedProxies(values []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(values))
	for _, raw := range values {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			addr, err := netip.ParseAddr(s)
			if err != nil {
				return nil, err
			}
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			out = append(out, netip.PrefixFrom(addr, bits))
			continue
		}
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, err
		}
		out = append(out, prefix)
	}
	return out, nil
}

func (r Resolver) trustsRemote(addr netip.Addr) bool {
	if len(r.TrustedProxies) == 0 || !addr.IsValid() {
		return false
	}
	for _, p := range r.TrustedProxies {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func (r Resolver) trustsRemoteAddr(remote string) bool {
	addr, _ := parseRemoteAddr(remote)
	return r.trustsRemote(addr)
}

func (r Resolver) usesForwarded() bool {
	return r.HeaderPolicy&HeaderPolicyForwarded != 0
}

func (r Resolver) usesXForwarded() bool {
	return r.HeaderPolicy&HeaderPolicyXForwarded != 0
}

func parseRemoteAddr(remote string) (netip.Addr, bool) {
	if remote == "" {
		return netip.Addr{}, false
	}
	host := remote
	if h, _, err := net.SplitHostPort(remote); err == nil {
		host = h
	}
	if host == "" {
		return netip.Addr{}, false
	}
	if i := strings.Index(host, "%"); i != -1 {
		host = host[:i]
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr, true
}

type forwarded struct {
	forAddr netip.Addr
	forOK   bool
	proto   string
	host    string
}

func forwardedFor(header string) (netip.Addr, bool) {
	f := parseForwarded(header)
	if f.forOK {
		return f.forAddr, true
	}
	return netip.Addr{}, false
}

func forwardedProto(header string) string {
	return parseForwarded(header).proto
}

func forwardedHost(header string) string {
	return parseForwarded(header).host
}

func parseForwarded(header string) forwarded {
	out := forwarded{}
	if header == "" {
		return out
	}
	parts := strings.Split(header, ",")
	entry := strings.TrimSpace(parts[0])
	if entry == "" {
		return out
	}
	params := strings.Split(entry, ";")
	for _, param := range params {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		key, val, ok := strings.Cut(param, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		val = strings.Trim(val, "\"")
		switch key {
		case "for":
			if addr, ok := parseAddrValue(val); ok {
				out.forAddr = addr
				out.forOK = true
			}
		case "proto":
			out.proto = strings.ToLower(val)
		case "host":
			out.host = val
		}
	}
	return out
}

func xForwardedFor(header string) (netip.Addr, bool) {
	if header == "" {
		return netip.Addr{}, false
	}
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return netip.Addr{}, false
	}
	return parseAddrValue(parts[0])
}

func xForwardedProto(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

func xForwardedHost(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func parseAddrValue(value string) (netip.Addr, bool) {
	val := strings.TrimSpace(value)
	if val == "" {
		return netip.Addr{}, false
	}
	val = strings.Trim(val, "\"")
	if val == "" {
		return netip.Addr{}, false
	}
	if strings.EqualFold(val, "unknown") || strings.HasPrefix(val, "_") {
		return netip.Addr{}, false
	}
	if strings.HasPrefix(val, "[") {
		if end := strings.Index(val, "]"); end > 1 {
			val = val[1:end]
		}
	}
	if addrPort, err := netip.ParseAddrPort(val); err == nil {
		return addrPort.Addr(), true
	}
	if host, _, err := net.SplitHostPort(val); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr, true
		}
	}
	if addr, err := netip.ParseAddr(val); err == nil {
		return addr, true
	}
	return netip.Addr{}, false
}
