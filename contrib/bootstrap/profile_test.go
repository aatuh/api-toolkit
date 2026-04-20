package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	requestlog "github.com/aatuh/api-toolkit/contrib/v2/middleware/requestlog"
	querylimits "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/prometheus/client_golang/prometheus"
)

type stubMiddlewareChain struct {
	middlewares []func(http.Handler) http.Handler
}

type captureErrorLogger struct {
	msg string
	kv  []any
}

type logEntry struct {
	level string
	msg   string
	kv    []any
}

type captureEntriesLogger struct {
	entries []logEntry
}

func (l *captureErrorLogger) Debug(string, ...any) {}
func (l *captureErrorLogger) Info(string, ...any)  {}
func (l *captureErrorLogger) Warn(string, ...any)  {}

func (l *captureErrorLogger) Error(msg string, kv ...any) {
	l.msg = msg
	l.kv = append([]any(nil), kv...)
}

func (l *captureEntriesLogger) Debug(msg string, kv ...any) {
	l.entries = append(l.entries, logEntry{level: "debug", msg: msg, kv: append([]any(nil), kv...)})
}

func (l *captureEntriesLogger) Info(msg string, kv ...any) {
	l.entries = append(l.entries, logEntry{level: "info", msg: msg, kv: append([]any(nil), kv...)})
}

func (l *captureEntriesLogger) Warn(msg string, kv ...any) {
	l.entries = append(l.entries, logEntry{level: "warn", msg: msg, kv: append([]any(nil), kv...)})
}

func (l *captureEntriesLogger) Error(msg string, kv ...any) {
	l.entries = append(l.entries, logEntry{level: "error", msg: msg, kv: append([]any(nil), kv...)})
}

type captureMetricsRecorder struct {
	counterCalls []metricsmw.Labels
	histCalls    []metricsmw.Labels
}

func (r *captureMetricsRecorder) IncCounter(_ string, labels metricsmw.Labels) {
	r.counterCalls = append(r.counterCalls, cloneMetricLabels(labels))
}

func (r *captureMetricsRecorder) ObserveHistogram(_ string, _ float64, labels metricsmw.Labels) {
	r.histCalls = append(r.histCalls, cloneMetricLabels(labels))
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

func TestProfileStrictAPIDisabledQueryLimitsSkipValidation(t *testing.T) {
	_, err := ProfileStrictAPI(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
		WithQueryLimitsOptions(querylimits.Options{MaxParams: -1}),
		WithQueryLimitsDisabled(),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
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

func TestProfileDevDisabledQueryLimitsSkipValidation(t *testing.T) {
	_, err := ProfileDev(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
		WithQueryLimitsOptions(querylimits.Options{MaxParams: -1}),
		WithQueryLimitsDisabled(),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
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

func TestProfileStrictAPIDoesNotRegisterPrometheusCollectorsByDefault(t *testing.T) {
	reg := withDefaultPrometheusRegistry(t)

	if _, err := ProfileStrictAPI(ports.NopLogger{}); err != nil {
		t.Fatalf("profile error: %v", err)
	}

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(metricFamilies) != 0 {
		t.Fatalf("expected no implicit collectors, got %d", len(metricFamilies))
	}
}

func TestProfileDevDoesNotRegisterPrometheusCollectorsByDefault(t *testing.T) {
	reg := withDefaultPrometheusRegistry(t)

	if _, err := ProfileDev(ports.NopLogger{}); err != nil {
		t.Fatalf("profile error: %v", err)
	}

	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(metricFamilies) != 0 {
		t.Fatalf("expected no implicit collectors, got %d", len(metricFamilies))
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

func TestProfileStrictAPIPanicBeforeCommitEmitsAccessLogAndMetrics(t *testing.T) {
	log := &captureEntriesLogger{}
	metrics := &captureMetricsRecorder{}
	profile, err := ProfileStrictAPI(
		log,
		WithMetricsRecorder(metrics),
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
	assertSingleAccessLog(t, log.entries, http.StatusInternalServerError)
	assertSingleMetricsObservation(t, metrics, "500")
}

func TestProfileStrictAPIPartialWritePanicEmitsAccessLogAndMetrics(t *testing.T) {
	log := &captureEntriesLogger{}
	metrics := &captureMetricsRecorder{}
	profile, err := ProfileStrictAPI(
		log,
		WithMetricsRecorder(metrics),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial:"))
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	defer func() {
		got := recover()
		err, ok := got.(error)
		if !ok || !errors.Is(err, http.ErrAbortHandler) {
			t.Fatalf("expected panic %v, got %v", http.ErrAbortHandler, got)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected recorder status 200, got %d", rec.Code)
		}
		assertSingleAccessLog(t, log.entries, http.StatusOK)
		assertSingleMetricsObservation(t, metrics, "200")
	}()
	handler.ServeHTTP(rec, req)
}

func TestProfileStrictAPIDoesNotAllowCrossOriginByDefault(t *testing.T) {
	profile, err := ProfileStrictAPI(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cors", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header, got %q", got)
	}
}

func TestProfileStrictAPIAppliesExplicitCORSAllowlist(t *testing.T) {
	profile, err := ProfileStrictAPI(
		ports.NopLogger{},
		WithMetricsRecorder(metricsmw.NoopMetrics{}),
		WithCORSOptions(ports.CORSOptions{
			AllowedOrigins: []string{"https://app.example.com"},
			AllowedMethods: []string{http.MethodGet, http.MethodOptions},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
			MaxAge:         300,
		}),
	)
	if err != nil {
		t.Fatalf("profile error: %v", err)
	}

	handler := wrapBootstrapProfile(profile, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cors", nil)
	req.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected explicit ACAO header, got %q", got)
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
var _ ports.Logger = (*captureEntriesLogger)(nil)

func assertSingleAccessLog(t *testing.T, entries []logEntry, wantStatus int) {
	t.Helper()
	var access []logEntry
	for _, entry := range entries {
		if entry.msg == "http" {
			access = append(access, entry)
		}
	}
	if len(access) != 1 {
		t.Fatalf("expected 1 access log entry, got %d", len(access))
	}
	if access[0].level != "error" {
		t.Fatalf("access log level = %q", access[0].level)
	}
	fields := kvToMap(access[0].kv)
	if fields[requestlog.FieldPanicRecovered] != true {
		t.Fatalf("panic_recovered = %v", fields[requestlog.FieldPanicRecovered])
	}
	if fields[requestlog.FieldStatus] != wantStatus {
		t.Fatalf("status = %v", fields[requestlog.FieldStatus])
	}
}

func assertSingleMetricsObservation(t *testing.T, recorder *captureMetricsRecorder, wantStatus string) {
	t.Helper()
	if len(recorder.counterCalls) != 1 {
		t.Fatalf("expected 1 counter call, got %d", len(recorder.counterCalls))
	}
	if len(recorder.histCalls) != 1 {
		t.Fatalf("expected 1 histogram call, got %d", len(recorder.histCalls))
	}
	if recorder.counterCalls[0]["status"] != wantStatus {
		t.Fatalf("counter status = %q", recorder.counterCalls[0]["status"])
	}
	if recorder.histCalls[0]["status"] != wantStatus {
		t.Fatalf("histogram status = %q", recorder.histCalls[0]["status"])
	}
}

func cloneMetricLabels(labels metricsmw.Labels) metricsmw.Labels {
	out := make(metricsmw.Labels, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

func withDefaultPrometheusRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()

	reg := prometheus.NewRegistry()
	prevRegisterer := prometheus.DefaultRegisterer
	prevGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = prevRegisterer
		prometheus.DefaultGatherer = prevGatherer
	})
	return reg
}
