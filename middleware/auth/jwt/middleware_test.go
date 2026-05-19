package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/ports"
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

func TestSubjectFromTokenEnforcesRequiredClaimsAndAlgorithms(t *testing.T) {
	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()
	mw := &Middleware{
		cfg: Config{
			Issuer:   "https://issuer.example",
			Audience: "example",
		},
		jwks:        kf,
		allowedAlgs: []string{"RS256"},
		claimReq: claimRequirements{
			requireSubject:    true,
			requireExpiration: true,
			requireIssuedAt:   true,
			requireNotBefore:  true,
		},
	}

	claims := baseClaims(now)
	claims["nbf"] = float64(now.Add(-time.Minute).Unix())
	token := signToken(t, jwt.SigningMethodRS256, claims, privateKey, "test-kid")
	subject, err := mw.subjectFromToken(token)
	if err != nil {
		t.Fatalf("subjectFromToken() error = %v", err)
	}
	if subject.UserID != "user" {
		t.Fatalf("subject user id = %q, want user", subject.UserID)
	}

	missingIssuedAt := baseClaims(now)
	delete(missingIssuedAt, "iat")
	missingIssuedAt["nbf"] = float64(now.Add(-time.Minute).Unix())
	token = signToken(t, jwt.SigningMethodRS256, missingIssuedAt, privateKey, "test-kid")
	if _, err := mw.subjectFromToken(token); err == nil || !strings.Contains(err.Error(), "token missing iat") {
		t.Fatalf("expected missing iat error, got %v", err)
	}

	hsToken := signToken(t, jwt.SigningMethodHS256, claims, []byte("secret"), "test-kid")
	if _, err := mw.subjectFromToken(hsToken); err == nil {
		t.Fatal("expected disallowed HS256 token to fail")
	}
}

func TestSubjectFromTokenRejectsInvalidRegisteredClaimsAndKeyMisses(t *testing.T) {
	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()
	mw := &Middleware{
		cfg: Config{
			Issuer:           "https://issuer.example",
			Audience:         "example",
			AllowedClockSkew: 5 * time.Second,
		},
		jwks:        kf,
		allowedAlgs: []string{"RS256"},
		claimReq: claimRequirements{
			requireSubject:    true,
			requireExpiration: true,
			requireIssuedAt:   true,
			requireNotBefore:  true,
		},
	}

	tests := []struct {
		name   string
		mutate func(jwt.MapClaims)
		kid    string
		want   string
	}{
		{
			name: "invalid issuer",
			mutate: func(claims jwt.MapClaims) {
				claims["iss"] = "https://evil.example"
			},
			kid:  "test-kid",
			want: "issuer",
		},
		{
			name: "invalid audience",
			mutate: func(claims jwt.MapClaims) {
				claims["aud"] = "other-api"
			},
			kid:  "test-kid",
			want: "audience",
		},
		{
			name: "expired token",
			mutate: func(claims jwt.MapClaims) {
				claims["exp"] = float64(now.Add(-time.Minute).Unix())
			},
			kid:  "test-kid",
			want: "expired",
		},
		{
			name: "not yet valid token",
			mutate: func(claims jwt.MapClaims) {
				claims["nbf"] = float64(now.Add(time.Minute).Unix())
			},
			kid:  "test-kid",
			want: "not valid",
		},
		{
			name: "missing subject",
			mutate: func(claims jwt.MapClaims) {
				delete(claims, "sub")
			},
			kid:  "test-kid",
			want: "subject",
		},
		{
			name:   "jwks key miss",
			mutate: func(jwt.MapClaims) {},
			kid:    "unknown-kid",
			want:   "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := baseClaims(now)
			claims["nbf"] = float64(now.Add(-time.Minute).Unix())
			tt.mutate(claims)
			token := signToken(t, jwt.SigningMethodRS256, claims, privateKey, tt.kid)
			_, err := mw.subjectFromToken(token)
			if err == nil {
				t.Fatal("expected token validation error")
			}
			if got := strings.ToLower(err.Error()); !strings.Contains(got, tt.want) {
				t.Fatalf("expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestOptionalHandlerAuthFlow(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	called := false
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected optional auth to allow missing token")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer malformed")
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for malformed token")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
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

func TestNewMiddlewareReportsJWKSSetupFailure(t *testing.T) {
	mw, err := NewMiddleware(context.Background(), Config{
		Enabled:  true,
		JWKSURL:  "://not-a-url",
		Issuer:   "https://issuer.example",
		Audience: "example",
	}, ports.NopLogger{})
	if err == nil {
		if mw != nil {
			mw.Close()
		}
		t.Fatal("expected JWKS setup error")
	}
	if !strings.Contains(err.Error(), "initializing jwks") {
		t.Fatalf("error = %q, want initializing jwks", err.Error())
	}
}

func newTestKeyfunc(t *testing.T) (keyfunc.Keyfunc, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	jwk, err := jwkset.NewJWKFromKey(privateKey.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{
			KID: "test-kid",
			ALG: jwkset.AlgRS256,
		},
	})
	if err != nil {
		t.Fatalf("create jwk: %v", err)
	}
	store := jwkset.NewMemoryStorage()
	if err := store.KeyWrite(context.Background(), jwk); err != nil {
		t.Fatalf("write jwk: %v", err)
	}
	kf, err := keyfunc.New(keyfunc.Options{Storage: store})
	if err != nil {
		t.Fatalf("create keyfunc: %v", err)
	}
	return kf, privateKey
}

func signToken(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, key any, kid string) string {
	t.Helper()

	token := jwt.NewWithClaims(method, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func baseClaims(now time.Time) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "user",
		"iss": "https://issuer.example",
		"aud": "example",
		"exp": float64(now.Add(time.Hour).Unix()),
		"iat": float64(now.Unix()),
	}
}
