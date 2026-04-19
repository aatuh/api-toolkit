package authtest

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
)

// ClaimRequirements mirrors the required-claim policy exercised in auth tests.
type ClaimRequirements struct {
	RequireSubject    bool
	RequireExpiration bool
	RequireIssuedAt   bool
	RequireNotBefore  bool
}

// RunBearerTokenCases verifies a shared malformed-header matrix.
func RunBearerTokenCases(t *testing.T, parse func(string) (string, bool, error)) {
	t.Helper()

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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			token, present, err := parse(tc.header)
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

// RunAlgorithmNormalizationCases verifies algorithm allowlist normalization rules.
func RunAlgorithmNormalizationCases(t *testing.T, normalize func([]string) ([]string, error)) {
	t.Helper()

	got, err := normalize([]string{"RS256", " rs256 ", "HS256", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"RS256", "HS256"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	if _, err := normalize([]string{"none"}); err == nil {
		t.Fatal("expected error for NONE algorithm")
	}
}

// RunClaimRequirementDefaultCases verifies the default claim policy.
func RunClaimRequirementDefaultCases[T any](
	t *testing.T,
	defaults func() T,
	toFlags func(T) ClaimRequirements,
) {
	t.Helper()

	got := toFlags(defaults())
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

// RunClaimValidationCases verifies a shared required-claims matrix.
func RunClaimValidationCases[T any](
	t *testing.T,
	defaults func() T,
	makeReq func(ClaimRequirements) T,
	validate func(jwt.MapClaims, T) error,
) {
	t.Helper()

	now := time.Now()
	exp := float64(now.Add(time.Hour).Unix())
	iat := float64(now.Unix())
	nbf := float64(now.Add(-time.Minute).Unix())

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		req     T
		wantErr bool
	}{
		{
			name:    "missing subject",
			claims:  jwt.MapClaims{"exp": exp},
			req:     defaults(),
			wantErr: true,
		},
		{
			name:    "missing expiration",
			claims:  jwt.MapClaims{"sub": "user"},
			req:     defaults(),
			wantErr: true,
		},
		{
			name:    "missing issued at",
			claims:  jwt.MapClaims{"sub": "user", "exp": exp},
			req:     makeReq(ClaimRequirements{RequireIssuedAt: true}),
			wantErr: true,
		},
		{
			name:    "missing not before",
			claims:  jwt.MapClaims{"sub": "user", "exp": exp},
			req:     makeReq(ClaimRequirements{RequireNotBefore: true}),
			wantErr: true,
		},
		{
			name: "all required present",
			claims: jwt.MapClaims{
				"sub": "user",
				"exp": exp,
				"iat": iat,
				"nbf": nbf,
			},
			req: makeReq(ClaimRequirements{
				RequireSubject:    true,
				RequireExpiration: true,
				RequireIssuedAt:   true,
				RequireNotBefore:  true,
			}),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validate(tc.claims, tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
		})
	}
}

// RunSkipHeaderCases verifies trusted-proxy skip-header behavior.
func RunSkipHeaderCases(t *testing.T, shouldSkip func(*http.Request) bool) {
	t.Helper()

	trustedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	trustedReq.Header.Set("X-Debug-Skip", "true")
	trustedReq.RemoteAddr = netip.MustParseAddr("203.0.113.7").String() + ":443"
	if !shouldSkip(trustedReq) {
		t.Fatal("expected trusted proxy request to skip auth")
	}

	untrustedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	untrustedReq.Header.Set("X-Debug-Skip", "true")
	untrustedReq.RemoteAddr = netip.MustParseAddr("198.51.100.7").String() + ":443"
	if shouldSkip(untrustedReq) {
		t.Fatal("expected untrusted proxy request to require auth")
	}
}

// RunSubjectAlgorithmCases verifies shared signing-algorithm enforcement.
func RunSubjectAlgorithmCases(
	t *testing.T,
	newParser func(t *testing.T, kf keyfunc.Keyfunc) func(string) (string, error),
) {
	t.Helper()

	kf, privateKey := newTestKeyfunc(t)
	parse := newParser(t, kf)
	now := time.Now()

	rsToken := signToken(t, jwt.SigningMethodRS256, baseClaims(now), privateKey, "test-kid")
	userID, err := parse(rsToken)
	if err != nil {
		t.Fatalf("expected RS256 token to parse: %v", err)
	}
	if userID != "user" {
		t.Fatalf("expected subject user id, got %q", userID)
	}

	hsToken := signToken(t, jwt.SigningMethodHS256, baseClaims(now), []byte("secret"), "test-kid")
	if _, err := parse(hsToken); err == nil {
		t.Fatal("expected HS256 token to be rejected")
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
