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

func wrapBootstrapProfile(profile Profile, next http.Handler) http.Handler {
	handler := next
	for i := len(profile.Middlewares) - 1; i >= 0; i-- {
		handler = profile.Middlewares[i](handler)
	}
	return handler
}
