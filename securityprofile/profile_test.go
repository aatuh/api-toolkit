package securityprofile

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

type stubMiddlewareChain struct {
	middlewares []func(http.Handler) http.Handler
}

func (s *stubMiddlewareChain) Use(middlewares ...func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middlewares...)
}

func TestRouteOverridesMaxBody(t *testing.T) {
	profile, err := New(
		WithRequireAuth(false),
		WithMaxBodyBytes(16),
		WithRouteOverrides(RouteOverride{
			Pattern:      "/upload",
			MaxBodyBytes: ptrInt64(128),
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	okReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", strings.NewReader(strings.Repeat("a", 64)))
	okRec := httptest.NewRecorder()
	handler.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected upload to pass, got %d", okRec.Code)
	}

	failReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/default", strings.NewReader(strings.Repeat("b", 64)))
	failRec := httptest.NewRecorder()
	handler.ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected default to fail, got %d", failRec.Code)
	}
}

func TestRouteOverridesTimeout(t *testing.T) {
	short := 20 * time.Millisecond
	long := 200 * time.Millisecond
	profile, err := New(
		WithRequireAuth(false),
		WithTimeout(short),
		WithRouteOverrides(RouteOverride{
			Pattern: "/slow",
			Timeout: &long,
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			http.Error(w, "timed out", http.StatusGatewayTimeout)
			return
		case <-time.After(60 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		}
	}))

	defaultReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/default", nil)
	defaultRec := httptest.NewRecorder()
	handler.ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected default timeout, got %d", defaultRec.Code)
	}

	slowReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/slow", nil)
	slowRec := httptest.NewRecorder()
	handler.ServeHTTP(slowRec, slowReq)
	if slowRec.Code != http.StatusOK {
		t.Fatalf("expected slow override to pass, got %d", slowRec.Code)
	}
}

func TestWithHardTimeoutWritesTimeoutProblem(t *testing.T) {
	profile, err := New(
		WithRequireAuth(false),
		WithHardTimeout(5*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected hard timeout 504, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/problem+json") {
		t.Fatalf("expected problem response, got content type %q", got)
	}
}

func TestRouteOverrideCanSelectHardTimeout(t *testing.T) {
	short := 5 * time.Millisecond
	hard := true
	profile, err := New(
		WithRequireAuth(false),
		WithTimeout(200*time.Millisecond),
		WithRouteOverrides(RouteOverride{
			Pattern:     "/hard",
			Timeout:     &short,
			HardTimeout: &hard,
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	hardReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/hard", nil)
	hardRec := httptest.NewRecorder()
	handler.ServeHTTP(hardRec, hardReq)
	if hardRec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected hard timeout route to return 504, got %d", hardRec.Code)
	}

	defaultReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/default", nil)
	defaultRec := httptest.NewRecorder()
	handler.ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusOK {
		t.Fatalf("expected cooperative default route to return 200, got %d", defaultRec.Code)
	}
}

func TestOWASPBaselineLimits(t *testing.T) {
	profile, err := OWASPBaseline(WithRequireAuth(false))
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}
	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	query := url.Values{}
	for i := 0; i < 101; i++ {
		query.Add("k"+strconv.Itoa(i), "1")
	}
	queryReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?"+query.Encode(), nil)
	queryRec := httptest.NewRecorder()
	handler.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected query limits to reject, got %d", queryRec.Code)
	}

	blocked := 0
	for i := 0; i < 100; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked == 0 {
		t.Fatalf("expected rate limiter to block at least one request")
	}
}

func TestProfileApplyToUsesMinimalMiddlewareChain(t *testing.T) {
	chain := &stubMiddlewareChain{}
	profile := Profile{
		Middlewares: []func(http.Handler) http.Handler{
			func(next http.Handler) http.Handler { return next },
			func(next http.Handler) http.Handler { return next },
		},
	}

	profile.ApplyTo(chain)

	if len(chain.middlewares) != 2 {
		t.Fatalf("expected 2 middlewares, got %d", len(chain.middlewares))
	}
}

func wrapProfile(profile Profile, next http.Handler) http.Handler {
	handler := next
	for i := len(profile.Middlewares) - 1; i >= 0; i-- {
		handler = profile.Middlewares[i](handler)
	}
	return handler
}

func ptrInt64(v int64) *int64 {
	return &v
}
