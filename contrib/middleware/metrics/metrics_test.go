package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"
	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
	timeoutmw "github.com/aatuh/api-toolkit/v3/middleware/timeout"
	"github.com/aatuh/api-toolkit/v3/ports"
	"github.com/aatuh/api-toolkit/v3/routecontracts"
	"github.com/aatuh/api-toolkit/v3/routepolicy"
	"github.com/aatuh/api-toolkit/v3/specs"
)

type captureRecorder struct {
	counterName string
	counter     Labels
	histName    string
	histValue   float64
	hist        Labels
}

func (r *captureRecorder) IncCounter(name string, labels Labels) {
	r.counterName = name
	r.counter = cloneLabels(labels)
}

func (r *captureRecorder) ObserveHistogram(name string, value float64, labels Labels) {
	r.histName = name
	r.histValue = value
	r.hist = cloneLabels(labels)
}

type capturePolicyRecorder struct {
	captureRecorder
	routePolicy Labels
}

func (r *capturePolicyRecorder) RecordRoutePolicy(labels Labels) {
	r.routePolicy = cloneLabels(labels)
}

func TestNewDefaults(t *testing.T) {
	mw, err := New(Options{})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	if mw.Clock == nil {
		t.Fatal("expected default clock")
	}
	if _, ok := mw.M.(NoopMetrics); !ok {
		t.Fatalf("expected NoopMetrics, got %T", mw.M)
	}
}

func TestNewPrometheusRecorderCheckedReusesRegisteredCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	first, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}
	second, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	if first.requests != second.requests {
		t.Fatal("expected duplicate counter registration to reuse existing collector")
	}
	if first.durations != second.durations {
		t.Fatal("expected duplicate histogram registration to reuse existing collector")
	}
	if first.healthStatusChanges != second.healthStatusChanges {
		t.Fatal("expected duplicate health status counter registration to reuse existing collector")
	}

	first.IncCounter("", Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})
	second.ObserveHistogram("", 0.25, Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 2 {
		t.Fatalf("expected 2 metric families, got %d", len(families))
	}
}

func TestNewPrometheusRecorderCheckedReusesRegisteredCollectorsWithDefaultRegisterer(t *testing.T) {
	reg := withDefaultPrometheusRegistry(t)

	first, err := NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}
	second, err := NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	if first.requests != second.requests {
		t.Fatal("expected duplicate counter registration to reuse existing collector")
	}
	if first.durations != second.durations {
		t.Fatal("expected duplicate histogram registration to reuse existing collector")
	}
	if first.healthStatusChanges != second.healthStatusChanges {
		t.Fatal("expected duplicate health status counter registration to reuse existing collector")
	}
	first.IncCounter("", Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})
	second.ObserveHistogram("", 0.25, Labels{
		"method": "GET",
		"route":  "/widgets",
		"status": "200",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	if len(families) != 2 {
		t.Fatalf("expected 2 metric families, got %d", len(families))
	}
}

func TestPrometheusRecorderRecordsHealthStatusChangesWithBoundedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	recorder.RecordHealthStatusChange(ports.HealthStatusHealthy, ports.HealthStatusUnhealthy, ports.DetailedHealthResponse{
		Status: ports.HealthStatusUnhealthy,
		Checks: map[string]ports.HealthResult{
			"database-primary-with-unbounded-name": {Status: ports.HealthStatusUnhealthy, Message: "dial tcp 10.0.0.1:5432: timeout"},
		},
	})
	recorder.RecordHealthStatusChange("unexpected-provider-status", "", ports.DetailedHealthResponse{})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	familyIndex := -1
	for i, candidate := range families {
		if candidate.GetName() == "health_status_changes_total" {
			familyIndex = i
			break
		}
	}
	if familyIndex == -1 {
		t.Fatal("health_status_changes_total was not gathered")
	}
	family := families[familyIndex]
	if len(family.Metric) != 2 {
		t.Fatalf("metric count = %d, want 2", len(family.Metric))
	}
	seen := map[string]bool{}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
			switch label.GetName() {
			case "from", "to":
			default:
				t.Fatalf("unexpected health metric label %q", label.GetName())
			}
			if strings.Contains(label.GetValue(), "database") || strings.Contains(label.GetValue(), "10.0.0.1") {
				t.Fatalf("health metric leaked high-cardinality value %q", label.GetValue())
			}
		}
		if len(labels) != 2 {
			t.Fatalf("labels = %#v, want only from/to", labels)
		}
		seen[labels["from"]+"->"+labels["to"]] = true
	}
	for _, want := range []string{"healthy->unhealthy", "other->unknown"} {
		if !seen[want] {
			t.Fatalf("missing health transition labels %s in %#v", want, seen)
		}
	}
}

func TestHealthStatusChangeHookRecordsTransitions(t *testing.T) {
	recorder := &captureHealthRecorder{}
	hook := HealthStatusChangeHook(recorder)

	hook(context.Background(), ports.HealthStatusDegraded, ports.HealthStatusHealthy, ports.DetailedHealthResponse{
		Status: ports.HealthStatusHealthy,
	})

	if recorder.from != ports.HealthStatusDegraded || recorder.to != ports.HealthStatusHealthy {
		t.Fatalf("recorded transition = %q -> %q", recorder.from, recorder.to)
	}
}

func TestPrometheusRecorderRecordsIdempotencyOutcomesWithBoundedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	recorder.RecordIdempotencyOutcome(idempotencymw.OutcomeEvent{
		Method:    "BREW",
		Status:    http.StatusCreated,
		StoreType: "customer-acme-memory-primary",
		Outcome:   idempotencymw.OutcomeEventName("customer-acme-outcome"),
		FailOpen:  true,
	})
	recorder.RecordIdempotencyOutcome(idempotencymw.OutcomeEvent{
		Method:    http.MethodPost,
		Status:    http.StatusServiceUnavailable,
		StoreType: "redis-cluster-customer-acme",
		Outcome:   idempotencymw.IdempotencyOutcomeReplayed,
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	family := metricFamilyByName(t, families, "idempotency_outcomes_total")
	if len(family.Metric) != 2 {
		t.Fatalf("metric count = %d, want 2", len(family.Metric))
	}
	seen := map[string]bool{}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
			switch label.GetName() {
			case "method", "store_class", "outcome", "status_class", "fail_open":
			default:
				t.Fatalf("unexpected idempotency metric label %q", label.GetName())
			}
			if strings.Contains(label.GetValue(), "customer") || strings.Contains(label.GetValue(), "acme") {
				t.Fatalf("idempotency metric leaked high-cardinality label %q", label.GetValue())
			}
		}
		if len(labels) != 5 {
			t.Fatalf("labels = %#v, want five bounded labels", labels)
		}
		seen[labels["method"]+"|"+labels["store_class"]+"|"+labels["outcome"]+"|"+labels["status_class"]+"|"+labels["fail_open"]] = true
	}
	for _, want := range []string{
		"OTHER|memory|unknown|2xx|true",
		"POST|redis|replayed|5xx|false",
	} {
		if !seen[want] {
			t.Fatalf("missing idempotency outcome labels %s in %#v", want, seen)
		}
	}
}

func TestIdempotencyOutcomeHookRecordsOutcomes(t *testing.T) {
	recorder := &captureIdempotencyOutcomeRecorder{}
	hook := IdempotencyOutcomeHook(recorder)

	hook(context.Background(), idempotencymw.OutcomeEvent{
		Method:  http.MethodPost,
		Status:  http.StatusCreated,
		Outcome: idempotencymw.IdempotencyOutcomeCompletedStored,
	})

	if recorder.event.Outcome != idempotencymw.IdempotencyOutcomeCompletedStored || recorder.event.Method != http.MethodPost {
		t.Fatalf("recorded event = %#v", recorder.event)
	}
}

func TestPrometheusRecorderRecordsHardTimeoutEventsWithBoundedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	recorder.RecordHardTimeoutEvent(timeoutmw.HardTimeoutEvent{
		Method:          "BREW",
		Status:          http.StatusGatewayTimeout,
		Outcome:         timeoutmw.HardTimeoutOutcome("customer-acme-timeout"),
		TimedOut:        true,
		Panicked:        true,
		CaptureOverflow: true,
	})
	recorder.RecordHardTimeoutEvent(timeoutmw.HardTimeoutEvent{
		Method:   http.MethodPost,
		Status:   http.StatusInternalServerError,
		Outcome:  timeoutmw.HardTimeoutOutcomeCaptureOverflow,
		TimedOut: false,
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	family := metricFamilyByName(t, families, "http_hard_timeout_events_total")
	if len(family.Metric) != 2 {
		t.Fatalf("metric count = %d, want 2", len(family.Metric))
	}
	seen := map[string]bool{}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
			switch label.GetName() {
			case "method", "outcome", "status_class", "timed_out", "panicked", "capture_overflow":
			default:
				t.Fatalf("unexpected hard-timeout metric label %q", label.GetName())
			}
			if strings.Contains(label.GetValue(), "customer") || strings.Contains(label.GetValue(), "acme") {
				t.Fatalf("hard-timeout metric leaked high-cardinality label %q", label.GetValue())
			}
		}
		if len(labels) != 6 {
			t.Fatalf("labels = %#v, want six bounded labels", labels)
		}
		seen[labels["method"]+"|"+labels["outcome"]+"|"+labels["status_class"]+"|"+labels["timed_out"]+"|"+labels["panicked"]+"|"+labels["capture_overflow"]] = true
	}
	for _, want := range []string{
		"OTHER|unknown|5xx|true|true|true",
		"POST|capture_overflow|5xx|false|false|false",
	} {
		if !seen[want] {
			t.Fatalf("missing hard-timeout labels %s in %#v", want, seen)
		}
	}
}

func TestHardTimeoutEventHookRecordsEvents(t *testing.T) {
	recorder := &captureHardTimeoutEventRecorder{}
	hook := HardTimeoutEventHook(recorder)

	hook(timeoutmw.HardTimeoutEvent{
		Method:  http.MethodGet,
		Status:  http.StatusGatewayTimeout,
		Outcome: timeoutmw.HardTimeoutOutcomeTimeout,
	})

	if recorder.event.Outcome != timeoutmw.HardTimeoutOutcomeTimeout || recorder.event.Method != http.MethodGet {
		t.Fatalf("recorded event = %#v", recorder.event)
	}
}

func TestPrometheusRecorderRecordsWebhookDeliveryWithBoundedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	recorder.ObserveWebhookDelivery(context.Background(), webhookdelivery.DeliveryObservation{
		EventType:   "Widget.Created",
		Outcome:     "accepted",
		StatusClass: "2xx",
	})
	recorder.ObserveWebhookDelivery(context.Background(), webhookdelivery.DeliveryObservation{
		EventType:   "customer acme deleted",
		Outcome:     "transport_error",
		StatusClass: "5xx",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	family := metricFamilyByName(t, families, "webhook_delivery_events_total")
	if len(family.Metric) != 2 {
		t.Fatalf("metric count = %d, want 2", len(family.Metric))
	}
	seen := map[string]bool{}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
			switch label.GetName() {
			case "event_type", "outcome", "status_class":
			default:
				t.Fatalf("unexpected webhook metric label %q", label.GetName())
			}
			if strings.Contains(label.GetValue(), " ") {
				t.Fatalf("webhook metric label was not normalized: %q", label.GetValue())
			}
		}
		if len(labels) != 3 {
			t.Fatalf("labels = %#v, want three bounded labels", labels)
		}
		seen[labels["event_type"]+"|"+labels["outcome"]+"|"+labels["status_class"]] = true
	}
	for _, want := range []string{
		"widget.created|accepted|2xx",
		"customer_acme_deleted|transport_error|5xx",
	} {
		if !seen[want] {
			t.Fatalf("missing webhook delivery labels %s in %#v", want, seen)
		}
	}
}

func TestWebhookDeliveryHookRecordsEvents(t *testing.T) {
	recorder := &captureWebhookDeliveryRecorder{}
	hook := WebhookDeliveryHook(recorder)

	hook.ObserveWebhookDelivery(context.Background(), webhookdelivery.DeliveryObservation{
		EventType:   "widget.created",
		Outcome:     "accepted",
		StatusClass: "2xx",
	})

	if recorder.event.Outcome != "accepted" || recorder.event.EventType != "widget.created" {
		t.Fatalf("recorded event = %#v", recorder.event)
	}
}

func TestHandlerRecordsRoutePolicyLabelsFromRouteContracts(t *testing.T) {
	recorder := &capturePolicyRecorder{}
	mw, err := New(Options{Recorder: recorder})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	router := chi.NewRouter()
	router.Use(mw.Middleware())
	registry := routecontracts.NewRegistry(router, nil)
	operation := routepolicy.ApplyMetadata(specs.Operation{Method: http.MethodPost, Path: "/widgets"},
		routepolicy.WithAuth("ApiKeyAuth", "widgets:write"),
		routepolicy.WithTenantRequired("header"),
		routepolicy.WithIdempotencyRequired(),
		routepolicy.WithRateLimit("write-standard"),
	)
	if err := registry.Post("/widgets", operation, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})); err != nil {
		t.Fatalf("register route: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if recorder.routePolicy["method"] != http.MethodPost || recorder.routePolicy["route"] != "/widgets" || recorder.routePolicy["status_class"] != "2xx" {
		t.Fatalf("route policy transport labels = %#v", recorder.routePolicy)
	}
	for key, want := range map[string]string{
		"auth":        "required",
		"tenant":      "required",
		"idempotency": "required",
		"rate_limit":  "configured",
		"admin":       "none",
		"deprecated":  "false",
	} {
		if got := recorder.routePolicy[key]; got != want {
			t.Fatalf("route policy label %s = %q, want %q; labels=%#v", key, got, want, recorder.routePolicy)
		}
	}
}

func TestRoutePolicyLabelsBoundsPolicyMetadata(t *testing.T) {
	labels := RoutePolicyLabels(Labels{
		"method":       "BREW",
		"route":        " /widgets/{id} ",
		"status":       "201",
		"status_class": "customer-acme-success",
		"auth":         "ApiKeyAuth",
		"tenant":       "tenant-acme",
		"idempotency":  "key-123",
		"rate_limit":   "gold-plan",
		"admin":        "root",
		"deprecated":   "yes",
	})

	want := Labels{
		"method":       "OTHER",
		"route":        "/widgets/{id}",
		"status_class": "2xx",
		"auth":         "none",
		"tenant":       "none",
		"idempotency":  "none",
		"rate_limit":   "none",
		"admin":        "none",
		"deprecated":   "false",
	}
	for key, wantValue := range want {
		if got := labels[key]; got != wantValue {
			t.Fatalf("label %s = %q, want %q; labels=%#v", key, got, wantValue, labels)
		}
	}
	for _, labelValue := range labels {
		if strings.Contains(labelValue, "acme") || strings.Contains(labelValue, "ApiKeyAuth") || strings.Contains(labelValue, "gold") {
			t.Fatalf("route policy label leaked high-cardinality policy data: %#v", labels)
		}
	}
}

func TestPrometheusRecorderRecordsRoutePolicyRequestsWithBoundedLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err != nil {
		t.Fatalf("new prometheus recorder: %v", err)
	}

	recorder.RecordRoutePolicy(Labels{
		"method":       "BREW",
		"route":        "/widgets/{id}",
		"status":       "201",
		"status_class": "customer-acme-success",
		"auth":         "ApiKeyAuth",
		"tenant":       "tenant-acme",
		"idempotency":  "key-123",
		"rate_limit":   "gold-plan",
		"admin":        "root",
		"deprecated":   "yes",
	})
	recorder.RecordRoutePolicy(Labels{
		"method":       http.MethodPost,
		"route":        "/widgets",
		"status":       "429",
		"status_class": "4xx",
		"auth":         "required",
		"tenant":       "required",
		"idempotency":  "required",
		"rate_limit":   "configured",
		"admin":        "required",
		"deprecated":   "true",
	})

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	family := metricFamilyByName(t, families, "http_route_policy_requests_total")
	if len(family.Metric) != 2 {
		t.Fatalf("metric count = %d, want 2", len(family.Metric))
	}
	seen := map[string]bool{}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, label := range metric.Label {
			labels[label.GetName()] = label.GetValue()
			switch label.GetName() {
			case "method", "route", "status_class", "auth", "tenant", "idempotency", "admin", "deprecated", "rate_limit":
			default:
				t.Fatalf("unexpected route-policy metric label %q", label.GetName())
			}
			if strings.Contains(label.GetValue(), "acme") || strings.Contains(label.GetValue(), "ApiKeyAuth") || strings.Contains(label.GetValue(), "gold") {
				t.Fatalf("route-policy metric leaked high-cardinality label %q", label.GetValue())
			}
		}
		if len(labels) != 9 {
			t.Fatalf("labels = %#v, want nine bounded labels", labels)
		}
		seen[labels["method"]+"|"+labels["route"]+"|"+labels["status_class"]+"|"+labels["auth"]+"|"+labels["tenant"]+"|"+labels["idempotency"]+"|"+labels["admin"]+"|"+labels["deprecated"]+"|"+labels["rate_limit"]] = true
	}
	for _, want := range []string{
		"OTHER|/widgets/{id}|2xx|none|none|none|none|false|none",
		"POST|/widgets|4xx|required|required|required|required|true|configured",
	} {
		if !seen[want] {
			t.Fatalf("missing route-policy labels %s in %#v", want, seen)
		}
	}
}

func TestNewPrometheusRecorderCheckedReturnsConflictErrorWithCustomRegisterer(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "route", "status"}))

	recorder, err := NewPrometheusRecorderChecked(reg, nil)
	if err == nil {
		t.Fatal("expected registration conflict error")
	}
	if recorder != nil {
		t.Fatal("expected recorder to be nil on registration conflict")
	}
	if !errors.Is(err, ErrIncompatibleCollectorRegistration) {
		t.Fatalf("expected ErrIncompatibleCollectorRegistration, got %v", err)
	}
	if !strings.Contains(err.Error(), "http_requests_total") {
		t.Fatalf("expected metric name in error, got %q", err)
	}
}

func TestNewPrometheusRecorderCheckedReturnsConflictErrorWithDefaultRegisterer(t *testing.T) {
	withDefaultPrometheusRegistry(t)
	prometheus.MustRegister(prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_request_duration_seconds",
		Help: "HTTP request duration in seconds",
	}, []string{"method", "route", "status"}))

	recorder, err := NewPrometheusRecorderChecked(nil, nil)
	if err == nil {
		t.Fatal("expected registration conflict error")
	}
	if recorder != nil {
		t.Fatal("expected recorder to be nil on registration conflict")
	}
	if !errors.Is(err, ErrIncompatibleCollectorRegistration) {
		t.Fatalf("expected ErrIncompatibleCollectorRegistration, got %v", err)
	}
	if !strings.Contains(err.Error(), "http_request_duration_seconds") {
		t.Fatalf("expected metric name in error, got %q", err)
	}
}

func TestSanitizeHTTPLabelsBoundsMethodAndStatus(t *testing.T) {
	method, route, status := sanitizeHTTPLabels(Labels{
		"method": "post",
		"route":  " /widgets/{id} ",
		"status": "201",
	})
	if method != "POST" || route != "/widgets/{id}" || status != "201" {
		t.Fatalf("labels = %q %q %q, want POST /widgets/{id} 201", method, route, status)
	}

	method, route, status = sanitizeHTTPLabels(Labels{
		"method": "BREW",
		"status": "599x",
	})
	if method != "OTHER" || route != "unknown" || status != "0" {
		t.Fatalf("labels = %q %q %q, want OTHER unknown 0", method, route, status)
	}
}

func TestHandlerRecordsRecoveredPanicAs500BeforeCommit(t *testing.T) {
	recorder := &captureRecorder{}
	mw, err := New(Options{Recorder: recorder})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	handler := mw.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	defer func() {
		if got := recover(); got != "boom" {
			t.Fatalf("expected panic boom, got %v", got)
		}
		if recorder.counterName != "http_requests_total" {
			t.Fatalf("counter name = %q", recorder.counterName)
		}
		if recorder.counter["status"] != "500" {
			t.Fatalf("counter status = %q", recorder.counter["status"])
		}
		if recorder.histName != "http_request_duration_seconds" {
			t.Fatalf("histogram name = %q", recorder.histName)
		}
		if recorder.hist["status"] != "500" {
			t.Fatalf("histogram status = %q", recorder.hist["status"])
		}
	}()
	handler.ServeHTTP(rec, req)
}

func cloneLabels(labels Labels) Labels {
	out := make(Labels, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

type captureHealthRecorder struct {
	from   ports.HealthStatus
	to     ports.HealthStatus
	result ports.DetailedHealthResponse
}

func (r *captureHealthRecorder) RecordHealthStatusChange(from, to ports.HealthStatus, result ports.DetailedHealthResponse) {
	r.from = from
	r.to = to
	r.result = result
}

type captureIdempotencyOutcomeRecorder struct {
	event idempotencymw.OutcomeEvent
}

func (r *captureIdempotencyOutcomeRecorder) RecordIdempotencyOutcome(event idempotencymw.OutcomeEvent) {
	r.event = event
}

type captureHardTimeoutEventRecorder struct {
	event timeoutmw.HardTimeoutEvent
}

func (r *captureHardTimeoutEventRecorder) RecordHardTimeoutEvent(event timeoutmw.HardTimeoutEvent) {
	r.event = event
}

type captureWebhookDeliveryRecorder struct {
	event webhookdelivery.DeliveryObservation
}

func (r *captureWebhookDeliveryRecorder) RecordWebhookDelivery(event webhookdelivery.DeliveryObservation) {
	r.event = event
}

func metricFamilyByName(t *testing.T, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("%s was not gathered", name)
	return nil
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
