package securityprofile

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/middleware/querylimits"
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
		WithHardTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
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

func TestWithHardTimeoutMaxCaptureBytesControlsOverflow(t *testing.T) {
	profile, err := New(
		WithRequireAuth(false),
		WithHardTimeout(time.Second),
		WithHardTimeoutMaxCaptureBytes(4),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Result", "oversized")
		_, _ = w.Write([]byte("too large"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected hard-timeout overflow 500, got %d", rec.Code)
	}
	if rec.Header().Get("X-Result") != "" {
		t.Fatalf("oversized handler header leaked: %q", rec.Header().Get("X-Result"))
	}
	if !strings.Contains(rec.Body.String(), "timeout-capture-overflow") {
		t.Fatalf("expected overflow problem response, got %q", rec.Body.String())
	}
}

func TestRouteOverrideCanSelectHardTimeout(t *testing.T) {
	short := 50 * time.Millisecond
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
		if r.URL.Path == "/hard" {
			<-r.Context().Done()
		}
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

func TestRouteOverrideCanTuneHardTimeoutCaptureBytes(t *testing.T) {
	limit := int64(4)
	profile, err := New(
		WithRequireAuth(false),
		WithHardTimeout(time.Second),
		WithRouteOverrides(RouteOverride{
			Pattern:                    "/small-capture",
			HardTimeoutMaxCaptureBytes: &limit,
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("small response"))
	}))

	defaultReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/default", nil)
	defaultRec := httptest.NewRecorder()
	handler.ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusOK {
		t.Fatalf("expected default route to use default capture limit, got %d", defaultRec.Code)
	}

	limitedReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/small-capture", nil)
	limitedRec := httptest.NewRecorder()
	handler.ServeHTTP(limitedRec, limitedReq)
	if limitedRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected route override overflow 500, got %d", limitedRec.Code)
	}
	if !strings.Contains(limitedRec.Body.String(), "timeout-capture-overflow") {
		t.Fatalf("expected overflow problem response, got %q", limitedRec.Body.String())
	}
}

func TestStreamingRouteOverrideDisablesHardTimeoutBuffering(t *testing.T) {
	profile, err := New(
		WithRequireAuth(false),
		WithHardTimeout(50*time.Millisecond),
		WithRouteOverrides(StreamingRouteOverride("/events", http.MethodGet)),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			writeOptionalResponseWriterEvidence(w)
			return
		} else {
			<-r.Context().Done()
		}
		w.WriteHeader(http.StatusOK)
	}))

	defaultReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/default", nil)
	defaultRec := httptest.NewRecorder()
	handler.ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected default route to keep hard timeout, got %d", defaultRec.Code)
	}

	streamReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/events", nil)
	streamRec := newOptionalResponseWriter()
	handler.ServeHTTP(streamRec, streamReq)
	if streamRec.Status() != http.StatusOK {
		t.Fatalf("expected streaming route to bypass hard timeout, got %d", streamRec.Status())
	}
	assertOptionalResponseWriterPreserved(t, streamRec)
	if streamRec.Body() != "stream-response" {
		t.Fatalf("expected streaming body through original writer, got %q", streamRec.Body())
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

func TestNewRequiresAuthCheckWhenAuthIsRequired(t *testing.T) {
	if _, err := New(); err == nil {
		t.Fatal("expected auth check requirement")
	}
}

func TestAuthAllowlistBypassesAuthCheck(t *testing.T) {
	profile, err := New(
		WithAuthCheck(func(*http.Request) bool { return false }),
		WithAuthAllowlist("/healthz", "/docs/*"),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}
	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/healthz", "/docs/openapi.json"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected allowlisted request to pass, got %d", rec.Code)
			}
		})
	}
}

func TestAuthCheckAndCustomErrorWriter(t *testing.T) {
	var capturedStatus int
	var capturedProblem map[string]any
	profile, err := New(
		WithAuthCheck(func(r *http.Request) bool {
			return r.Header.Get("Authorization") == "Bearer ok"
		}),
		WithErrorWriter(func(w http.ResponseWriter, status int, p httpx.Problem) {
			capturedStatus = status
			capturedProblem = map[string]any{
				"type":   p.Type,
				"title":  p.Title,
				"detail": p.Detail,
			}
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(capturedProblem)
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}
	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	failRec := httptest.NewRecorder()
	failReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/private", nil)
	handler.ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", failRec.Code)
	}
	if capturedStatus != http.StatusUnauthorized {
		t.Fatalf("captured status = %d, want 401", capturedStatus)
	}
	if capturedProblem["detail"] != "authentication required" {
		t.Fatalf("captured problem = %#v", capturedProblem)
	}

	okRec := httptest.NewRecorder()
	okReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/private", nil)
	okReq.Header.Set("Authorization", "Bearer ok")
	handler.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusNoContent {
		t.Fatalf("expected authorized request to pass, got %d", okRec.Code)
	}
}

func TestDevBypassDoesNotBypassWithoutDevBuildOrTrustedProxy(t *testing.T) {
	profile, err := New(
		WithAuthCheck(func(*http.Request) bool { return false }),
		WithDevBypassHeader("X-Debug-Bypass", true),
		WithResolver(identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyNone,
			TrustedProxies: []netip.Prefix{
				netip.MustParsePrefix("127.0.0.1/32"),
			},
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}
	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/private", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Debug-Bypass", "true")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected bypass to fail closed, got %d", rec.Code)
	}
}

func TestRouteOverrideValidationAndMethodMatching(t *testing.T) {
	if _, err := New(WithRequireAuth(false), WithRouteOverrides(RouteOverride{})); err == nil {
		t.Fatal("expected empty route override pattern to fail")
	}

	disabled := false
	profile, err := New(
		WithRequireAuth(false),
		WithQueryLimits(querylimits.Options{MaxParams: 1}),
		WithRouteOverrides(RouteOverride{
			Pattern:            "/search",
			Methods:            []string{" post "},
			QueryLimitsEnabled: &disabled,
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}
	handler := wrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	postRec := httptest.NewRecorder()
	postReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/search?a=1&b=2", nil)
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusNoContent {
		t.Fatalf("expected POST override to disable query limits, got %d", postRec.Code)
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/search?a=1&b=2", nil)
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusBadRequest {
		t.Fatalf("expected GET to keep baseline query limits, got %d", getRec.Code)
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

type optionalResponseWriter struct {
	header        http.Header
	status        int
	body          strings.Builder
	flushed       bool
	pushedTargets []string
	readFromCalls int
}

func newOptionalResponseWriter() *optionalResponseWriter {
	return &optionalResponseWriter{header: make(http.Header)}
}

func (w *optionalResponseWriter) Header() http.Header {
	return w.header
}

func (w *optionalResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *optionalResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *optionalResponseWriter) Flush() {
	w.flushed = true
}

func (w *optionalResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, net.ErrClosed
}

func (w *optionalResponseWriter) Push(target string, _ *http.PushOptions) error {
	w.pushedTargets = append(w.pushedTargets, target)
	return nil
}

func (w *optionalResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.readFromCalls++
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return io.Copy(&w.body, r)
}

func (w *optionalResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *optionalResponseWriter) Body() string {
	return w.body.String()
}

func writeOptionalResponseWriterEvidence(w http.ResponseWriter) {
	flusher, hasFlusher := w.(http.Flusher)
	_, hasHijacker := w.(http.Hijacker)
	pusher, hasPusher := w.(http.Pusher)
	readerFrom, hasReaderFrom := w.(io.ReaderFrom)
	w.Header().Set("X-Has-Flusher", strconv.FormatBool(hasFlusher))
	w.Header().Set("X-Has-Hijacker", strconv.FormatBool(hasHijacker))
	w.Header().Set("X-Has-Pusher", strconv.FormatBool(hasPusher))
	w.Header().Set("X-Has-ReaderFrom", strconv.FormatBool(hasReaderFrom))
	w.WriteHeader(http.StatusOK)
	if hasFlusher {
		flusher.Flush()
	}
	if hasPusher {
		_ = pusher.Push("/events/next", nil)
	}
	if hasReaderFrom {
		_, _ = readerFrom.ReadFrom(strings.NewReader("stream-response"))
		return
	}
	_, _ = w.Write([]byte("stream-response"))
}

func assertOptionalResponseWriterPreserved(t *testing.T, w *optionalResponseWriter) {
	t.Helper()
	for name, value := range map[string]string{
		"X-Has-Flusher":    "true",
		"X-Has-Hijacker":   "true",
		"X-Has-Pusher":     "true",
		"X-Has-ReaderFrom": "true",
	} {
		if got := w.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if !w.flushed {
		t.Fatal("expected streaming route to call Flush on original writer")
	}
	if len(w.pushedTargets) != 1 || w.pushedTargets[0] != "/events/next" {
		t.Fatalf("pushed targets = %v, want [/events/next]", w.pushedTargets)
	}
	if w.readFromCalls != 1 {
		t.Fatalf("ReadFrom calls = %d, want 1", w.readFromCalls)
	}
}
