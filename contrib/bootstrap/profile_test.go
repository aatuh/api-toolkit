package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	"github.com/aatuh/api-toolkit/v2/ports"
)

type stubMiddlewareChain struct {
	middlewares []func(http.Handler) http.Handler
}

type captureErrorLogger struct {
	msg string
	kv  []any
}

func (l *captureErrorLogger) Debug(string, ...any) {}
func (l *captureErrorLogger) Info(string, ...any)  {}
func (l *captureErrorLogger) Warn(string, ...any)  {}

func (l *captureErrorLogger) Error(msg string, kv ...any) {
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (s *stubMiddlewareChain) Use(middlewares ...func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middlewares...)
}

func TestProfileStrictAPIEnforcesQueryLimitsByDefault(t *testing.T) {
	profile, err := ProfileStrictAPI(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	query := url.Values{}
	for i := 0; i < 101; i++ {
		query.Add("k"+strconv.Itoa(i), "1")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem response, got %q", got)
	}
}

func TestProfileStrictAPICanDisableQueryLimits(t *testing.T) {
	profile, err := ProfileStrictAPI(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
		WithQueryLimitsDisabled(),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	query := url.Values{}
	for i := 0; i < 101; i++ {
		query.Add("k"+strconv.Itoa(i), "1")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestProfileDevDoesNotEnforceQueryLimitsByDefault(t *testing.T) {
	profile, err := ProfileDev(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	query := url.Values{}
	for i := 0; i < 101; i++ {
		query.Add("k"+strconv.Itoa(i), "1")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestProfileStrictAPICanBeConstructedRepeatedlyWithDefaultMetrics(t *testing.T) {
	for i := 0; i < 2; i++ {
		profile, err := ProfileStrictAPI(ports.NopLogger{})
		if err != nil {
			t.Fatalf("profile error on iteration %d: %v", i, err)
		}
		if len(profile.Middlewares) == 0 {
			t.Fatalf("expected middleware stack on iteration %d", i)
		}
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

func TestProfileStrictAPILogsRecoveredPanicsWithStack(t *testing.T) {
	log := &captureErrorLogger{}
	profile, err := ProfileStrictAPI(
		log,
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if log.msg != "panic recovered" {
		t.Fatalf("unexpected log message %q", log.msg)
	}
	fields := kvToMap(log.kv)
	if fields["panic"] != "boom" {
		t.Fatalf("panic field = %v", fields["panic"])
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatal("expected stack field")
	}
}

func wrapBootstrapProfile(profile Profile, next http.Handler) http.Handler {
	handler := next
	for i := len(profile.Middlewares) - 1; i >= 0; i-- {
		handler = profile.Middlewares[i](handler)
	}
	return handler
}

func kvToMap(kv []any) map[string]any {
	out := make(map[string]any)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || key == "" {
			continue
		}
		out[key] = kv[i+1]
	}
	return out
}

var _ ports.Logger = (*captureErrorLogger)(nil)
