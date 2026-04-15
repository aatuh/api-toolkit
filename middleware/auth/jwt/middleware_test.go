package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestHandlerReturnsUnauthorizedWhenTokenMissing(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandlerReturnsUnauthorizedWhenAuthorizationHeaderMalformed(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestParseBearerTokenRejectsMalformedHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "multiple values", header: "Bearer token, Bearer other"},
		{name: "surrounding whitespace", header: " Bearer token"},
		{name: "wrong scheme", header: "Basic abc123"},
		{name: "token whitespace", header: "Bearer token with-space"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, present, err := parseBearerToken(tt.header)
			if err == nil {
				t.Fatal("expected error")
			}
			if !present {
				t.Fatal("expected header to be treated as present")
			}
			if token != "" {
				t.Fatalf("expected empty token, got %q", token)
			}
		})
	}
}

func TestValidateRequiredClaimsHonorsConfiguredRequirements(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		req    claimRequirements
		claims jwt.MapClaims
		want   string
	}{
		{
			name: "missing issued at",
			req: claimRequirements{
				requireSubject:    true,
				requireExpiration: true,
				requireIssuedAt:   true,
			},
			claims: jwt.MapClaims{
				"sub": "user_123",
				"exp": float64(now.Add(time.Hour).Unix()),
			},
			want: "token missing iat",
		},
		{
			name: "missing not before",
			req: claimRequirements{
				requireSubject:    true,
				requireExpiration: true,
				requireNotBefore:  true,
			},
			claims: jwt.MapClaims{
				"sub": "user_123",
				"exp": float64(now.Add(time.Hour).Unix()),
			},
			want: "token missing nbf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredClaims(tt.claims, tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestShouldSkipRequiresTrustedProxyAndBypassFlags(t *testing.T) {
	prefixes, err := identity.ParseTrustedProxies([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}

	mw := &Middleware{
		cfg: Config{
			SkipHeaderEnabled:         true,
			AllowDangerousDevBypasses: true,
		},
		skipHdr:      "X-Debug-Skip",
		skipResolver: identity.Resolver{TrustedProxies: prefixes},
	}

	trustedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	trustedReq.Header.Set("X-Debug-Skip", "true")
	trustedReq.RemoteAddr = netip.MustParseAddr("203.0.113.7").String() + ":443"
	if !mw.shouldSkip(trustedReq) {
		t.Fatal("expected trusted proxy request to skip auth")
	}

	untrustedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	untrustedReq.Header.Set("X-Debug-Skip", "true")
	untrustedReq.RemoteAddr = netip.MustParseAddr("198.51.100.7").String() + ":443"
	if mw.shouldSkip(untrustedReq) {
		t.Fatal("expected untrusted proxy request to require auth")
	}
}

func TestNewMiddlewareValidatesConfiguration(t *testing.T) {
	base := Config{
		Enabled:  true,
		JWKSURL:  "https://example.test/.well-known/jwks.json",
		Issuer:   "https://issuer.test/",
		Audience: "api-toolkit",
	}

	tests := []struct {
		name          string
		useNilContext bool
		cfg           Config
		want          string
	}{
		{
			name:          "requires context when enabled",
			useNilContext: true,
			cfg:           base,
			want:          "context is required",
		},
		{
			name: "requires mandatory config",
			cfg:  Config{Enabled: true},
			want: "jwt middleware missing mandatory configuration",
		},
		{
			name: "rejects none algorithm",
			cfg: func() Config {
				cfg := base
				cfg.AllowedAlgorithms = []string{"none"}
				return cfg
			}(),
			want: "algorithm none is not allowed",
		},
		{
			name: "rejects invalid trusted proxy config",
			cfg: func() Config {
				cfg := base
				cfg.AllowDangerousDevBypasses = true
				cfg.SkipHeaderEnabled = true
				cfg.SkipHeaderName = "X-Debug-Skip"
				cfg.SkipTrustedProxies = []string{"not-a-cidr"}
				return cfg
			}(),
			want: "jwt skip trusted proxies",
		},
		{
			name: "requires trusted proxies for skip header",
			cfg: func() Config {
				cfg := base
				cfg.AllowDangerousDevBypasses = true
				cfg.SkipHeaderEnabled = true
				cfg.SkipHeaderName = "X-Debug-Skip"
				return cfg
			}(),
			want: "jwt skip header requires trusted proxies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.useNilContext {
				ctx = nil
			}
			mw, err := NewMiddleware(ctx, tt.cfg, ports.NopLogger{})
			if err == nil {
				if mw != nil {
					mw.Close()
				}
				t.Fatal("expected error")
			}
			if got := err.Error(); got == "" || !strings.Contains(got, tt.want) {
				t.Fatalf("expected error containing %q, got %q", tt.want, got)
			}
		})
	}
}
