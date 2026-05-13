package clerk

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

	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestParseBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		header  string
		token   string
		present bool
		wantErr bool
	}{
		{
			name:    "missing",
			header:  "",
			token:   "",
			present: false,
			wantErr: false,
		},
		{
			name:    "valid",
			header:  "Bearer abc.def",
			token:   "abc.def",
			present: true,
			wantErr: false,
		},
		{
			name:    "valid-lowercase",
			header:  "bearer token",
			token:   "token",
			present: true,
			wantErr: false,
		},
		{
			name:    "leading-space",
			header:  " Bearer token",
			token:   "",
			present: true,
			wantErr: true,
		},
		{
			name:    "trailing-space",
			header:  "Bearer token ",
			token:   "",
			present: true,
			wantErr: true,
		},
		{
			name:    "multiple-values",
			header:  "Bearer one, Bearer two",
			token:   "",
			present: true,
			wantErr: true,
		},
		{
			name:    "wrong-scheme",
			header:  "Basic token",
			token:   "",
			present: true,
			wantErr: true,
		},
		{
			name:    "empty-token",
			header:  "Bearer ",
			token:   "",
			present: true,
			wantErr: true,
		},
		{
			name:    "token-whitespace",
			header:  "Bearer abc def",
			token:   "",
			present: true,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token, present, err := parseBearerToken(tc.header)
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

func TestNormalizeAlgorithms(t *testing.T) {
	t.Parallel()

	got, err := normalizeAlgorithms([]string{"RS256", " rs256 ", "HS256", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"RS256", "HS256"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	if _, err := normalizeAlgorithms([]string{"none"}); err == nil {
		t.Fatal("expected error for NONE algorithm")
	}
}

func TestNormalizeClaimRequirementsDefaults(t *testing.T) {
	t.Parallel()

	got := normalizeClaimRequirements(ClaimRequirements{})
	if !got.requireSubject {
		t.Fatal("expected subject requirement enabled by default")
	}
	if !got.requireExpiration {
		t.Fatal("expected expiration requirement enabled by default")
	}
	if got.requireIssuedAt {
		t.Fatal("expected issued-at requirement disabled by default")
	}
	if got.requireNotBefore {
		t.Fatal("expected not-before requirement disabled by default")
	}
}

func TestValidateRequiredClaims(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exp := float64(now.Add(time.Hour).Unix())
	iat := float64(now.Unix())
	nbf := float64(now.Add(-time.Minute).Unix())

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		req     claimRequirements
		wantErr bool
	}{
		{
			name:    "missing-subject",
			claims:  jwt.MapClaims{"exp": exp},
			req:     normalizeClaimRequirements(ClaimRequirements{}),
			wantErr: true,
		},
		{
			name:    "missing-expiration",
			claims:  jwt.MapClaims{"sub": "user"},
			req:     normalizeClaimRequirements(ClaimRequirements{}),
			wantErr: true,
		},
		{
			name:    "missing-issued-at",
			claims:  jwt.MapClaims{"sub": "user", "exp": exp},
			req:     claimRequirements{requireIssuedAt: true},
			wantErr: true,
		},
		{
			name:    "missing-not-before",
			claims:  jwt.MapClaims{"sub": "user", "exp": exp},
			req:     claimRequirements{requireNotBefore: true},
			wantErr: true,
		},
		{
			name: "all-required-present",
			claims: jwt.MapClaims{
				"sub": "user",
				"exp": exp,
				"iat": iat,
				"nbf": nbf,
			},
			req: claimRequirements{
				requireSubject:    true,
				requireExpiration: true,
				requireIssuedAt:   true,
				requireNotBefore:  true,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateRequiredClaims(tc.claims, tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
		})
	}
}

func TestSubjectFromTokenEnforcesAllowedAlgorithms(t *testing.T) {
	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()

	mw := &Middleware{
		cfg: Config{
			Issuer:   "https://issuer.example",
			Audience: "example",
		},
		jwks:        kf,
		allowedAlgs: []string{"RS256"},
		claimReq:    normalizeClaimRequirements(ClaimRequirements{}),
	}

	rsToken := signToken(t, jwt.SigningMethodRS256, baseClaims(now), privateKey, "test-kid")
	subject, err := mw.subjectFromToken(rsToken)
	if err != nil {
		t.Fatalf("expected RS256 token to parse: %v", err)
	}
	if subject.UserID != "user" {
		t.Fatalf("expected subject user id, got %q", subject.UserID)
	}
	if subject.TenantID != "tenant_1" {
		t.Fatalf("expected tenant id, got %q", subject.TenantID)
	}
	if subject.Scope != "widgets:write" {
		t.Fatalf("expected scope claim, got %q", subject.Scope)
	}

	hsToken := signToken(t, jwt.SigningMethodHS256, baseClaims(now), []byte("secret"), "test-kid")
	if _, err := mw.subjectFromToken(hsToken); err == nil {
		t.Fatal("expected HS256 token to be rejected")
	}
}

func TestRequiredAndOptionalHandlerFlows(t *testing.T) {
	mw := &Middleware{enabled: true, log: ports.NopLogger{}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("required handler should not run without a token")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("required missing-token status = %d, want 401", rec.Code)
	}

	called := false
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if !called {
		t.Fatal("expected optional handler to allow missing token")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("optional missing-token status = %d, want 204", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer malformed")
	mw.OptionalHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("optional handler should reject malformed token")
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("optional malformed-token status = %d, want 401", rec.Code)
	}
}

func TestShouldSkipDeniesUnsafeBypassCases(t *testing.T) {
	prefixes, err := identity.ParseTrustedProxies([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}
	newMiddleware := func() *Middleware {
		return &Middleware{
			cfg: Config{
				SkipHeaderEnabled:         true,
				AllowDangerousDevBypasses: true,
			},
			skipHdr:      "X-Debug-Skip",
			skipResolver: identity.Resolver{TrustedProxies: prefixes},
		}
	}
	trustedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	trustedReq.Header.Set("X-Debug-Skip", "true")
	trustedReq.RemoteAddr = netip.MustParseAddr("203.0.113.7").String() + ":443"

	tests := []struct {
		name   string
		mutate func(*Middleware, *http.Request)
	}{
		{
			name: "dangerous bypass flag disabled",
			mutate: func(mw *Middleware, _ *http.Request) {
				mw.cfg.AllowDangerousDevBypasses = false
			},
		},
		{
			name: "skip header disabled",
			mutate: func(mw *Middleware, _ *http.Request) {
				mw.cfg.SkipHeaderEnabled = false
			},
		},
		{
			name: "header value not true",
			mutate: func(_ *Middleware, req *http.Request) {
				req.Header.Set("X-Debug-Skip", "false")
			},
		},
		{
			name: "trusted proxies missing",
			mutate: func(mw *Middleware, _ *http.Request) {
				mw.skipResolver = identity.Resolver{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := newMiddleware()
			req := trustedReq.Clone(context.Background())
			req.Header = trustedReq.Header.Clone()
			req.RemoteAddr = trustedReq.RemoteAddr
			tt.mutate(mw, req)
			if mw.shouldSkip(req) {
				t.Fatal("expected skip header to be denied")
			}
		})
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
		"sub":       "user",
		"iss":       "https://issuer.example",
		"aud":       "example",
		"exp":       float64(now.Add(time.Hour).Unix()),
		"iat":       float64(now.Unix()),
		"scope":     "widgets:write",
		"tenant_id": "tenant_1",
	}
}
