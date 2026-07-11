package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/healthchecktest"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestResolveJWKSURLFromDiscoveryValidatesIssuer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Fatalf("unexpected discovery path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":   "https://issuer.example",
			"jwks_uri": "https://issuer.example/jwks.json",
		})
	}))
	defer server.Close()

	jwksURL, err := ResolveJWKSURL(context.Background(), Config{
		Enabled:      true,
		Issuer:       "https://issuer.example",
		DiscoveryURL: server.URL + "/.well-known/openid-configuration",
	}, server.Client())
	if err != nil {
		t.Fatalf("ResolveJWKSURL() error = %v", err)
	}
	if jwksURL != "https://issuer.example/jwks.json" {
		t.Fatalf("jwksURL = %q", jwksURL)
	}

	_, err = ResolveJWKSURL(context.Background(), Config{
		Enabled:      true,
		Issuer:       "https://wrong.example",
		DiscoveryURL: server.URL + "/.well-known/openid-configuration",
	}, server.Client())
	if err == nil || !strings.Contains(err.Error(), "issuer mismatch") {
		t.Fatalf("expected issuer mismatch, got %v", err)
	}
}

func TestSubjectFromTokenUsesConfiguredTenantAndScopeClaims(t *testing.T) {
	t.Parallel()

	kf, privateKey := newTestKeyfunc(t)
	now := time.Now()
	mw := &Middleware{
		cfg: Config{
			Issuer:      "https://issuer.example",
			Audience:    "example",
			TenantClaim: "organization_id",
			ScopeClaim:  "permissions",
		},
		jwks:        kf,
		allowedAlgs: []string{"RS256"},
		claimReq:    normalizeClaimRequirements(ClaimRequirements{}),
	}
	claims := baseClaims(now)
	claims["organization_id"] = "org_123"
	claims["permissions"] = []string{"widgets:read", "widgets:write"}
	claims["email"] = "user@example.com"
	token := signToken(t, jwt.SigningMethodRS256, claims, privateKey, "test-kid")
	//nolint:staticcheck // Regression coverage for the nil-context fail-closed guard.
	if _, err := mw.subjectFromTokenContext(nil, token); err == nil || err.Error() != "oidc jwt verification context is required" {
		t.Fatalf("nil verification context error = %v", err)
	}

	subject, err := mw.subjectFromToken(token)
	if err != nil {
		t.Fatalf("subjectFromToken() error = %v", err)
	}
	if subject.UserID != "user" || subject.Email != "user@example.com" || subject.TenantID != "org_123" {
		t.Fatalf("subject = %#v", subject)
	}
	if subject.Scope != "widgets:read widgets:write" {
		t.Fatalf("scope = %q", subject.Scope)
	}
	if subject.Claims["organization_id"] != "org_123" {
		t.Fatalf("claims not copied: %#v", subject.Claims)
	}
	claims["organization_id"] = "changed"
	if subject.Claims["organization_id"] != "org_123" {
		t.Fatalf("claims alias original map: %#v", subject.Claims)
	}
}

func TestNewMiddlewareDiscoversJWKSAndAuthenticates(t *testing.T) {
	t.Parallel()

	privateKey, jwksJSON := newTestJWKSet(t)
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":   issuer,
				"jwks_uri": issuer + "/jwks.json",
			})
		case "/jwks.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(jwksJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	issuer = server.URL

	mw, err := NewMiddleware(context.Background(), Config{
		Enabled:      true,
		Issuer:       issuer,
		Audience:     "example",
		DiscoveryURL: issuer + "/.well-known/openid-configuration",
		TenantClaim:  "tenant_id",
	}, ports.NopLogger{})
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	defer mw.Close()

	token := signToken(t, jwt.SigningMethodRS256, baseClaimsWithIssuer(time.Now(), issuer), privateKey, "test-kid")
	var got Subject
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		got, ok = SubjectFromContext(r.Context())
		if !ok {
			t.Fatal("missing subject in context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got.UserID != "user" || got.TenantID != "tenant_1" {
		t.Fatalf("subject = %#v", got)
	}
}

func TestNewMiddlewareFailClosedConfigAndDisabled(t *testing.T) {
	t.Parallel()

	mw, err := NewMiddleware(context.Background(), Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("disabled NewMiddleware() error = %v", err)
	}
	mw.Close()
	mw.Close()

	_, err = NewMiddleware(context.Background(), Config{Enabled: true, Audience: "example"}, ports.NopLogger{})
	if err == nil || !strings.Contains(err.Error(), "oidc middleware missing mandatory configuration") {
		t.Fatalf("expected mandatory config error, got %v", err)
	}
	_, err = NewMiddleware(context.Background(), Config{
		Enabled:      true,
		Issuer:       "https://issuer.example",
		Audience:     "example",
		JWKSURL:      "https://issuer.example/jwks.json",
		TenantClaim:  "tenant_id",
		ScopeClaim:   "scope",
		DiscoveryURL: "",
		AllowedAlgorithms: []string{
			"none",
		},
	}, ports.NopLogger{})
	if err == nil || !strings.Contains(err.Error(), "algorithm none is not allowed") {
		t.Fatalf("expected algorithm rejection, got %v", err)
	}
}

func TestHealthCheckerContract(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	checker := HealthChecker(Config{Enabled: true, JWKSURL: server.URL, JWKSRefreshTimeout: time.Second}, server.Client())
	healthchecktest.AssertCheckerContract(t, checker, "oidc", health.StatusHealthy)
	if HealthChecker(Config{Enabled: false, JWKSURL: server.URL}, nil) != nil {
		t.Fatal("disabled health checker should be nil")
	}
}

func TestSubjectContextAndDisabledHandlers(t *testing.T) {
	t.Parallel()

	ctx := WithSubject(context.Background(), Subject{UserID: "user_1", TenantID: "org_1"})
	subject, ok := SubjectFromContext(ctx)
	if !ok || subject.UserID != "user_1" || subject.TenantID != "org_1" {
		t.Fatalf("SubjectFromContext() = %#v, %v", subject, ok)
	}
	if _, ok := SubjectFromContext(context.Background()); ok {
		t.Fatal("expected empty context to have no subject")
	}

	mw := &Middleware{enabled: false, log: ports.NopLogger{}}
	called := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("disabled handler called=%v status=%d", called, rec.Code)
	}
	var nilMiddleware *Middleware
	if nilMiddleware.Handler(http.NotFoundHandler()) == nil || nilMiddleware.OptionalHandler(http.NotFoundHandler()) == nil {
		t.Fatal("nil middleware should pass handlers through")
	}
	nilMiddleware.Close()
}

func newTestKeyfunc(t *testing.T) (keyfunc.Keyfunc, *rsa.PrivateKey) {
	t.Helper()

	privateKey, jwksJSON := newTestJWKSet(t)
	var marshaled jwkset.JWKSMarshal
	if err := json.Unmarshal(jwksJSON, &marshaled); err != nil {
		t.Fatalf("unmarshal jwks: %v", err)
	}
	store, err := marshaled.ToStorage()
	if err != nil {
		t.Fatalf("jwks to storage: %v", err)
	}
	kf, err := keyfunc.New(keyfunc.Options{Storage: store})
	if err != nil {
		t.Fatalf("create keyfunc: %v", err)
	}
	return kf, privateKey
}

func newTestJWKSet(t *testing.T) (*rsa.PrivateKey, []byte) {
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
	jwksJSON, err := store.JSONPublic(context.Background())
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	return privateKey, jwksJSON
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
	return baseClaimsWithIssuer(now, "https://issuer.example")
}

func baseClaimsWithIssuer(now time.Time, issuer string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub":       "user",
		"iss":       issuer,
		"aud":       "example",
		"exp":       float64(now.Add(time.Hour).Unix()),
		"iat":       float64(now.Unix()),
		"tenant_id": "tenant_1",
		"scope":     "widgets:write",
	}
}
