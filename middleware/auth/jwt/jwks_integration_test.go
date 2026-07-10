package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestJWTJWKIntegrationRotatesKeysOnUnknownKID(t *testing.T) {
	oldKey := newJWKSIntegrationKey(t, "kid-old")
	newKey := newJWKSIntegrationKey(t, "kid-new")
	server, setJWKS := newMutableJWKSServer(t, jwksIntegrationJSON(t, oldKey))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mw := newJWKSIntegrationMiddleware(t, ctx, server.URL)

	now := time.Now()
	oldToken := signToken(t, jwt.SigningMethodRS256, baseClaims(now), oldKey.privateKey, oldKey.kid)
	assertJWTIntegrationStatus(t, mw, context.Background(), oldToken, http.StatusNoContent)

	setJWKS(jwksIntegrationJSON(t, newKey))
	newToken := signToken(t, jwt.SigningMethodRS256, baseClaims(now), newKey.privateKey, newKey.kid)
	assertJWTIntegrationStatus(t, mw, context.Background(), newToken, http.StatusNoContent)
}

func TestJWTJWKIntegrationRejectsInvalidAlgorithmMissingKIDAndStaleCache(t *testing.T) {
	key := newJWKSIntegrationKey(t, "kid-known")
	unknownKey := newJWKSIntegrationKey(t, "kid-unknown")
	server, _ := newMutableJWKSServer(t, jwksIntegrationJSON(t, key))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mw := newJWKSIntegrationMiddleware(t, ctx, server.URL)
	now := time.Now()

	t.Run("invalid alg", func(t *testing.T) {
		token := signToken(t, jwt.SigningMethodHS256, baseClaims(now), []byte("secret"), key.kid)
		assertJWTIntegrationStatus(t, mw, context.Background(), token, http.StatusUnauthorized)
	})

	t.Run("missing kid", func(t *testing.T) {
		token := signToken(t, jwt.SigningMethodRS256, baseClaims(now), key.privateKey, "")
		assertJWTIntegrationStatus(t, mw, context.Background(), token, http.StatusUnauthorized)
	})

	t.Run("stale cache unknown kid", func(t *testing.T) {
		token := signToken(t, jwt.SigningMethodRS256, baseClaims(now), unknownKey.privateKey, unknownKey.kid)
		assertJWTIntegrationStatus(t, mw, context.Background(), token, http.StatusUnauthorized)
	})
}

func TestJWTJWKIntegrationRequestCancellationStopsUnknownKIDRefresh(t *testing.T) {
	cachedKey := newJWKSIntegrationKey(t, "kid-cached")
	rotatedKey := newJWKSIntegrationKey(t, "kid-rotated")
	server, setJWKS := newMutableJWKSServer(t, jwksIntegrationJSON(t, cachedKey))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mw := newJWKSIntegrationMiddleware(t, ctx, server.URL)

	setJWKS(jwksIntegrationJSON(t, rotatedKey))
	token := signToken(t, jwt.SigningMethodRS256, baseClaims(time.Now()), rotatedKey.privateKey, rotatedKey.kid)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	assertJWTIntegrationStatus(t, mw, requestCtx, token, http.StatusUnauthorized)
}

func TestJWTJWKIntegrationNetworkFailureFailsClosed(t *testing.T) {
	key := newJWKSIntegrationKey(t, "kid-network")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "jwks unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	mw := newJWKSIntegrationMiddleware(t, ctx, server.URL)

	token := signToken(t, jwt.SigningMethodRS256, baseClaims(time.Now()), key.privateKey, key.kid)
	assertJWTIntegrationStatus(t, mw, context.Background(), token, http.StatusUnauthorized)
}

type jwksIntegrationKey struct {
	kid        string
	privateKey *rsa.PrivateKey
}

func newJWKSIntegrationKey(t *testing.T, kid string) jwksIntegrationKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return jwksIntegrationKey{kid: kid, privateKey: privateKey}
}

func jwksIntegrationJSON(t *testing.T, keys ...jwksIntegrationKey) []byte {
	t.Helper()
	store := jwkset.NewMemoryStorage()
	for _, key := range keys {
		jwk, err := jwkset.NewJWKFromKey(key.privateKey.Public(), jwkset.JWKOptions{
			Metadata: jwkset.JWKMetadataOptions{
				KID: key.kid,
				ALG: jwkset.AlgRS256,
			},
		})
		if err != nil {
			t.Fatalf("create jwk %q: %v", key.kid, err)
		}
		if err := store.KeyWrite(context.Background(), jwk); err != nil {
			t.Fatalf("write jwk %q: %v", key.kid, err)
		}
	}
	raw, err := store.JSONPublic(context.Background())
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}
	return raw
}

func newMutableJWKSServer(t *testing.T, initial []byte) (*httptest.Server, func([]byte)) {
	t.Helper()
	var mu sync.RWMutex
	current := append([]byte(nil), initial...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(current)
	}))
	t.Cleanup(server.Close)
	setJWKS := func(next []byte) {
		mu.Lock()
		defer mu.Unlock()
		current = append([]byte(nil), next...)
	}
	return server, setJWKS
}

func newJWKSIntegrationMiddleware(t *testing.T, ctx context.Context, jwksURL string) *Middleware {
	t.Helper()
	mw, err := NewMiddleware(ctx, Config{
		Enabled:             true,
		JWKSURL:             jwksURL,
		Issuer:              "https://issuer.example",
		Audience:            "example",
		AllowedAlgorithms:   []string{"RS256"},
		JWKSRefreshTimeout:  time.Second,
		JWKSRefreshInterval: time.Hour,
	}, ports.NopLogger{})
	if err != nil {
		t.Fatalf("NewMiddleware() error = %v", err)
	}
	t.Cleanup(mw.Close)
	return mw
}

func assertJWTIntegrationStatus(t *testing.T, mw *Middleware, ctx context.Context, token string, wantStatus int) {
	t.Helper()
	called := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	if wantStatus == http.StatusNoContent {
		if !called {
			t.Fatal("handler did not run for accepted token")
		}
		return
	}
	if called {
		t.Fatalf("handler ran for rejected token with status %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("rejection content type = %q", got)
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatal("rejection body leaked raw token")
	}
}
