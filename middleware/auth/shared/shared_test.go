package shared

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		token   string
		present bool
		wantErr bool
	}{
		{name: "missing", header: "", token: "", present: false, wantErr: false},
		{name: "valid", header: "Bearer abc.def", token: "abc.def", present: true, wantErr: false},
		{name: "valid lowercase", header: "bearer token", token: "token", present: true, wantErr: false},
		{name: "leading whitespace", header: " Bearer token", token: "", present: true, wantErr: true},
		{name: "trailing whitespace", header: "Bearer token ", token: "", present: true, wantErr: true},
		{name: "multiple values", header: "Bearer one, Bearer two", token: "", present: true, wantErr: true},
		{name: "wrong scheme", header: "Basic token", token: "", present: true, wantErr: true},
		{name: "empty token", header: "Bearer ", token: "", present: true, wantErr: true},
		{name: "token whitespace", header: "Bearer abc def", token: "", present: true, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, present, err := ParseBearerToken(tc.header)
			if present != tc.present {
				t.Fatalf("present mismatch: got %v want %v", present, tc.present)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
			if token != tc.token {
				t.Fatalf("token mismatch: got %q want %q", token, tc.token)
			}
		})
	}
}

func TestParseBearerTokenValuesRejectsDuplicateAuthorizationHeaders(t *testing.T) {
	token, present, err := ParseBearerTokenValues([]string{"Bearer valid", "Bearer attacker"})
	if err == nil {
		t.Fatal("expected duplicate authorization header error")
	}
	if !present {
		t.Fatal("expected duplicate authorization headers to count as present")
	}
	if token != "" {
		t.Fatalf("expected empty token for duplicate authorization headers, got %q", token)
	}
}

func TestNormalizeAlgorithms(t *testing.T) {
	got, err := NormalizeAlgorithms([]string{"RS256", " rs256 ", "HS256", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"RS256", "HS256"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	if _, err := NormalizeAlgorithms([]string{"none"}); err == nil {
		t.Fatal("expected error for NONE algorithm")
	}
}

func TestNormalizeClaimRequirementsDefaults(t *testing.T) {
	got := NormalizeClaimRequirements(ClaimRequirementsInput{})
	if !got.RequireSubject {
		t.Fatal("expected subject requirement enabled by default")
	}
	if !got.RequireExpiration {
		t.Fatal("expected expiration requirement enabled by default")
	}
	if got.RequireIssuedAt {
		t.Fatal("expected issued-at requirement disabled by default")
	}
	if got.RequireNotBefore {
		t.Fatal("expected not-before requirement disabled by default")
	}
}

func TestValidateRequiredClaims(t *testing.T) {
	now := time.Now()
	exp := float64(now.Add(time.Hour).Unix())
	iat := float64(now.Unix())
	nbf := float64(now.Add(-time.Minute).Unix())

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		req     ClaimRequirements
		wantErr bool
	}{
		{name: "missing subject", claims: jwt.MapClaims{"exp": exp}, req: NormalizeClaimRequirements(ClaimRequirementsInput{}), wantErr: true},
		{name: "missing expiration", claims: jwt.MapClaims{"sub": "user"}, req: NormalizeClaimRequirements(ClaimRequirementsInput{}), wantErr: true},
		{name: "missing issued at", claims: jwt.MapClaims{"sub": "user", "exp": exp}, req: ClaimRequirements{RequireIssuedAt: true}, wantErr: true},
		{name: "missing not before", claims: jwt.MapClaims{"sub": "user", "exp": exp}, req: ClaimRequirements{RequireNotBefore: true}, wantErr: true},
		{
			name: "all required present",
			claims: jwt.MapClaims{
				"sub": "user",
				"exp": exp,
				"iat": iat,
				"nbf": nbf,
			},
			req: ClaimRequirements{
				RequireSubject:    true,
				RequireExpiration: true,
				RequireIssuedAt:   true,
				RequireNotBefore:  true,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRequiredClaims(tc.claims, tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
		})
	}
}

func TestShouldSkipRequestRequiresTrustedProxyAndFlags(t *testing.T) {
	prefixes, err := identity.ParseTrustedProxies([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}

	policy := SkipPolicy{
		Enabled:                   true,
		AllowDangerousDevBypasses: true,
		HeaderName:                "X-Debug-Skip",
		Resolver:                  identity.Resolver{TrustedProxies: prefixes},
	}

	trustedReq := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	trustedReq.Header.Set("X-Debug-Skip", "true")
	trustedReq.RemoteAddr = netip.MustParseAddr("203.0.113.7").String() + ":443"
	if !ShouldSkipRequest(trustedReq, policy) {
		t.Fatal("expected trusted proxy request to skip auth")
	}

	untrustedReq := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	untrustedReq.Header.Set("X-Debug-Skip", "true")
	untrustedReq.RemoteAddr = netip.MustParseAddr("198.51.100.7").String() + ":443"
	if ShouldSkipRequest(untrustedReq, policy) {
		t.Fatal("expected untrusted proxy request to require auth")
	}

	if ShouldSkipRequest(nil, policy) {
		t.Fatal("expected nil request not to skip auth")
	}
}

func TestParseSkipTrustedProxiesRequiresConfiguredCIDRs(t *testing.T) {
	if _, err := ParseSkipTrustedProxies([]string{"not-a-cidr"}); err == nil {
		t.Fatal("expected invalid trusted proxy error")
	}
	if _, err := ParseSkipTrustedProxies(nil); err == nil {
		t.Fatal("expected empty trusted proxy error")
	}
}

func TestParseTokenClaimsEnforcesAllowedAlgorithms(t *testing.T) {
	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()
	cfg := TokenParserConfig{
		Issuer:            "https://issuer.example",
		Audience:          "example",
		AllowedAlgorithms: []string{"RS256"},
		Requirements:      NormalizeClaimRequirements(ClaimRequirementsInput{}),
	}

	rsToken := signToken(t, jwt.SigningMethodRS256, baseClaims(now), privateKey, "test-kid")
	claims, err := ParseTokenClaims(rsToken, kf.Keyfunc, cfg)
	if err != nil {
		t.Fatalf("expected RS256 token to parse: %v", err)
	}
	if got := StringClaim(claims, "sub"); got != "user" {
		t.Fatalf("expected subject claim, got %q", got)
	}

	hsToken := signToken(t, jwt.SigningMethodHS256, baseClaims(now), []byte("secret"), "test-kid")
	if _, err := ParseTokenClaims(hsToken, kf.Keyfunc, cfg); err == nil {
		t.Fatal("expected HS256 token to be rejected")
	}
}

func TestRequiredBearerHandlerRejectsMissingToken(t *testing.T) {
	called := false
	handler := RequiredBearerHandler(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			called = true
		}),
		true,
		ports.NopLogger{},
		nil,
		HandlerMessages{
			MissingDetail: "missing",
			InvalidDetail: "invalid",
		},
		func(*http.Request) (string, bool, error) {
			return "", false, nil
		},
		func(ctx context.Context, _ string) (context.Context, error) {
			return ctx, nil
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("expected next handler not to run")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestOptionalBearerHandlerAllowsMissingToken(t *testing.T) {
	called := false
	handler := OptionalBearerHandler(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}),
		true,
		ports.NopLogger{},
		nil,
		HandlerMessages{
			MissingDetail: "missing",
			InvalidDetail: "invalid",
		},
		func(*http.Request) (string, bool, error) {
			return "", false, nil
		},
		func(ctx context.Context, _ string) (context.Context, error) {
			return ctx, nil
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to run")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
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
