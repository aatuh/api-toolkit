package webhookdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/async"
	"github.com/aatuh/api-toolkit/v4/webhooks"
)

func TestCatalogRejectsUnsupportedEvents(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog("widget.created")
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if err := catalog.ValidateEvent(Event{
		ID:         "evt_1",
		TenantID:   "org_1",
		Type:       "widget.deleted",
		Payload:    json.RawMessage(`{"id":"w_1"}`),
		OccurredAt: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	}); !errors.Is(err, ErrUnsupportedEvent) {
		t.Fatalf("ValidateEvent() error = %v, want %v", err, ErrUnsupportedEvent)
	}
}

func TestCatalogValidationBranches(t *testing.T) {
	t.Parallel()

	if _, err := NewCatalog(); !errors.Is(err, ErrUnsupportedEvent) {
		t.Fatalf("NewCatalog() error = %v, want %v", err, ErrUnsupportedEvent)
	}
	if _, err := NewCatalog("bad event"); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("NewCatalog(bad) error = %v, want %v", err, ErrInvalidEvent)
	}
	catalog, err := NewCatalog(" widget.created ")
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if !catalog.Allows("widget.created") || !catalog.Allows(" widget.created ") {
		t.Fatalf("Allows() should trim event names consistently")
	}
	if err := ValidateEvent(Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`not json`)}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("ValidateEvent() error = %v, want %v", err, ErrInvalidEvent)
	}
}

func TestValidateEndpointRequiresSafeHTTPSWebhookTarget(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{
		ID:            "we_1",
		TenantID:      "org_1",
		URL:           "https://example.com/hooks",
		SigningSecret: []byte("super-secret"),
		Events:        []string{"widget.created"},
	}
	if err := ValidateEndpoint(endpoint, EndpointPolicy{}); err != nil {
		t.Fatalf("ValidateEndpoint() error = %v", err)
	}

	tests := []struct {
		name     string
		endpoint Endpoint
		want     error
	}{
		{name: "http rejected by default", endpoint: withEndpointURL(endpoint, "http://example.com/hooks"), want: ErrInvalidEndpoint},
		{name: "userinfo rejected", endpoint: withEndpointURL(endpoint, "https://secret@example.com/hooks"), want: ErrInvalidEndpoint},
		{name: "missing secret", endpoint: withEndpointSecret(endpoint, nil), want: ErrInvalidEndpoint},
		{name: "unsafe header", endpoint: withEndpointHeaders(endpoint, http.Header{"Authorization": []string{"Bearer token"}}), want: ErrUnsafeHeader},
		{name: "bad event name", endpoint: withEndpointEvents(endpoint, []string{"widget created"}), want: ErrInvalidEvent},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateEndpoint(tt.endpoint, EndpointPolicy{}); !errors.Is(err, tt.want) {
				t.Fatalf("ValidateEndpoint() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSafeHeadersCloneAndRejectUnsafeValues(t *testing.T) {
	t.Parallel()

	headers := http.Header{"X-Trace": []string{" abc "}}
	cloned, err := SafeHeaders(headers)
	if err != nil {
		t.Fatalf("SafeHeaders() error = %v", err)
	}
	headers.Set("X-Trace", "changed")
	if got := cloned.Get("X-Trace"); got != "abc" {
		t.Fatalf("cloned header = %q, want trimmed immutable copy", got)
	}
	if cloned, err := SafeHeaders(nil); err != nil || cloned == nil {
		t.Fatalf("SafeHeaders(nil) = %#v, %v", cloned, err)
	}
	for _, headers := range []http.Header{
		{"X-Bad\nName": []string{"value"}},
		{"X-Trace": []string{"bad\nvalue"}},
		{"X-Webhook-Signature": []string{"sig"}},
	} {
		if _, err := SafeHeaders(headers); !errors.Is(err, ErrUnsafeHeader) {
			t.Fatalf("SafeHeaders(%#v) error = %v, want %v", headers, err, ErrUnsafeHeader)
		}
	}
}

func TestEndpointMatchesTenantAndSubscription(t *testing.T) {
	t.Parallel()

	endpoint := Endpoint{ID: "we_1", TenantID: "org_1", Events: []string{"widget.created"}}
	event := Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created"}
	if !EndpointMatches(endpoint, event) {
		t.Fatal("EndpointMatches() = false, want true")
	}
	if EndpointMatches(endpoint, Event{ID: "evt_1", TenantID: "org_2", Type: "widget.created"}) {
		t.Fatal("EndpointMatches() should reject tenant mismatch")
	}
	if EndpointMatches(endpoint, Event{ID: "evt_1", TenantID: "org_1", Type: "widget.deleted"}) {
		t.Fatal("EndpointMatches() should reject unsubscribed events")
	}
	if EndpointMatches(Endpoint{ID: "we_1", TenantID: "org_1", Disabled: true, Events: []string{AnyEventType}}, event) {
		t.Fatal("EndpointMatches() should reject disabled endpoints")
	}
}

func TestDefaultDeliveryIDAndSafeLabelAreStableAndBounded(t *testing.T) {
	t.Parallel()

	event := Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created"}
	endpoint := Endpoint{ID: "we_1", TenantID: "org_1"}
	first := DefaultDeliveryID(event, endpoint)
	second := DefaultDeliveryID(event, endpoint)
	if first == "" || first != second || !strings.HasPrefix(first, "whdel_") {
		t.Fatalf("DefaultDeliveryID() = %q and %q", first, second)
	}
	if first == DefaultDeliveryID(event, Endpoint{ID: "we_2", TenantID: "org_1"}) {
		t.Fatalf("DefaultDeliveryID() should vary by endpoint")
	}
	if got := SafeLabel(" Weird Event/Type With Spaces And A Very Long Label That Should Be Truncated Past Sixty Four Bytes "); got != "weird_event_type_with_spaces_and_a_very_long_label_that_should_b" {
		t.Fatalf("SafeLabel() = %q", got)
	}
	if got := SafeLabel("!!!"); got != "___" {
		t.Fatalf("SafeLabel punctuation = %q", got)
	}
	if got := SafeLabel(""); got != "unknown" {
		t.Fatalf("SafeLabel empty = %q", got)
	}
}

func TestDispatcherEnqueuesTenantScopedDeliveries(t *testing.T) {
	t.Parallel()

	clock := fixedClock()
	catalog, err := NewCatalog("widget.created")
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	registry := &fakeRegistry{endpoints: []Endpoint{
		{ID: "we_1", TenantID: "org_1", URL: "https://example.com/a", SigningSecret: []byte("secret-a"), Events: []string{"widget.created"}},
		{ID: "we_2", TenantID: "org_2", URL: "https://example.com/b", SigningSecret: []byte("secret-b"), Events: []string{"widget.created"}},
		{ID: "we_3", TenantID: "org_1", URL: "https://example.com/c", SigningSecret: []byte("secret-c"), Events: []string{"widget.deleted"}},
	}}
	store := &fakeDeliveryStore{}
	dispatcher, err := NewDispatcher(DispatcherConfig{
		Catalog:        catalog,
		Endpoints:      registry,
		Store:          store,
		Clock:          clock,
		DeliveryIDFunc: func(Event, Endpoint) string { return "del_1" },
	})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	deliveries, err := dispatcher.Dispatch(context.Background(), Event{
		ID:       "evt_1",
		TenantID: "org_1",
		Type:     "widget.created",
		Payload:  json.RawMessage(`{"id":"w_1"}`),
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].EndpointID != "we_1" || deliveries[0].State != StatePending {
		t.Fatalf("Dispatch() deliveries = %#v", deliveries)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("enqueued = %d, want 1", len(store.enqueued))
	}
	if store.enqueued[0].job.EndpointID != "we_1" || store.enqueued[0].job.Event.TenantID != "org_1" {
		t.Fatalf("job payload = %#v", store.enqueued[0].job)
	}
	if !store.enqueued[0].delivery.CreatedAt.Equal(clock()) || !store.enqueued[0].delivery.NextAt.Equal(clock()) {
		t.Fatalf("delivery timestamps = %#v", store.enqueued[0].delivery)
	}
}

func TestDispatcherValidationAndStoreErrors(t *testing.T) {
	t.Parallel()

	catalog, err := NewCatalog("widget.created")
	if err != nil {
		t.Fatalf("NewCatalog() error = %v", err)
	}
	if _, err := NewDispatcher(DispatcherConfig{Catalog: catalog}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("NewDispatcher() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	if _, err := NewDispatcher(DispatcherConfig{Endpoints: &fakeRegistry{}, Store: &fakeDeliveryStore{}}); !errors.Is(err, ErrUnsupportedEvent) {
		t.Fatalf("NewDispatcher() empty catalog error = %v, want %v", err, ErrUnsupportedEvent)
	}
	dispatcher, err := NewDispatcher(DispatcherConfig{
		Catalog:   catalog,
		Endpoints: &fakeRegistry{err: errors.New("registry down")},
		Store:     &fakeDeliveryStore{},
	})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}); err == nil || !strings.Contains(err.Error(), "registry down") {
		t.Fatalf("Dispatch() registry error = %v", err)
	}

	store := &fakeDeliveryStore{}
	dispatcher, err = NewDispatcher(DispatcherConfig{
		Catalog:        catalog,
		Endpoints:      &fakeRegistry{endpoints: []Endpoint{{ID: "we_1", TenantID: "org_1", URL: "https://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}}},
		Store:          store,
		DeliveryIDFunc: func(Event, Endpoint) string { return "" },
	})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("Dispatch() invalid delivery error = %v, want %v", err, ErrInvalidDelivery)
	}

	store.err = errors.New("enqueue failed")
	dispatcher, err = NewDispatcher(DispatcherConfig{
		Catalog:   catalog,
		Endpoints: &fakeRegistry{endpoints: []Endpoint{{ID: "we_1", TenantID: "org_1", URL: "https://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}}},
		Store:     store,
	})
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	if _, err := dispatcher.Dispatch(context.Background(), Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}); err == nil || !strings.Contains(err.Error(), "enqueue failed") {
		t.Fatalf("Dispatch() enqueue error = %v", err)
	}
	if _, err := (*Dispatcher)(nil).Dispatch(context.Background(), Event{}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil Dispatch() error = %v, want %v", err, ErrStoreNotConfigured)
	}
}

func TestDelivererSignsRequestAndAccepts2xx(t *testing.T) {
	t.Parallel()

	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		gotBody = body
		if r.Header.Get("X-Webhook-Event-ID") != "evt_1" {
			t.Fatalf("event id header = %q", r.Header.Get("X-Webhook-Event-ID"))
		}
		if r.Header.Get("X-Webhook-Timestamp") != "2026-05-02T12:00:00Z" {
			t.Fatalf("timestamp header = %q", r.Header.Get("X-Webhook-Timestamp"))
		}
		verifier, err := webhooks.NewHMACSHA256Verifier(webhooks.HMACConfig{Secret: []byte("super-secret"), Prefix: "sha256="})
		if err != nil {
			t.Fatalf("NewHMACSHA256Verifier() error = %v", err)
		}
		if err := verifier.VerifyWebhook(r.Context(), r, body); err != nil {
			t.Fatalf("VerifyWebhook() error = %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	deliverer, err := NewDeliverer(DelivererConfig{
		Client:         server.Client(),
		Clock:          fixedClock(),
		EndpointPolicy: EndpointPolicy{AllowInsecureHTTP: true},
	})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	result, err := deliverer.Deliver(context.Background(), Endpoint{
		ID:            "we_1",
		TenantID:      "org_1",
		URL:           server.URL,
		SigningSecret: []byte("super-secret"),
		Events:        []string{"widget.created"},
		Headers:       http.Header{"X-Consumer": []string{"widgets"}},
	}, Event{
		ID:       "evt_1",
		TenantID: "org_1",
		Type:     "widget.created",
		Payload:  json.RawMessage(`{"id":"w_1"}`),
	}, 2)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if !result.Accepted || result.StatusCode != http.StatusAccepted || result.Attempt != 2 {
		t.Fatalf("Deliver() result = %#v", result)
	}
	if !strings.Contains(string(gotBody), `"payload":{"id":"w_1"}`) {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestDelivererClassifiesTransportAndStatusFailures(t *testing.T) {
	t.Parallel()

	var observations []DeliveryObservation
	deliverer, err := NewDeliverer(DelivererConfig{
		Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: secret=should-not-leak")
		}),
		Clock:   fixedClock(),
		Metrics: MetricsRecorderFunc(func(_ context.Context, event DeliveryObservation) { observations = append(observations, event) }),
	})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	result, err := deliverer.Deliver(context.Background(), Endpoint{
		ID:            "we_1",
		TenantID:      "org_1",
		URL:           "https://example.com/hooks",
		SigningSecret: []byte("super-secret"),
		Events:        []string{"widget.created"},
	}, Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"secret":"payload"}`)}, 0)
	if !errors.Is(err, ErrDeliveryFailed) || !result.Retryable || result.StatusClass != "transport" || result.Attempt != 1 {
		t.Fatalf("transport result=%#v err=%v", result, err)
	}
	if strings.Contains(result.Error, "secret") || strings.Contains(err.Error(), "payload") {
		t.Fatalf("unsafe transport error leaked sensitive content result=%#v err=%v", result, err)
	}
	if len(observations) != 1 || observations[0].Outcome != "transport_error" || observations[0].StatusClass != "transport" {
		t.Fatalf("observations = %#v", observations)
	}
	statusTests := []struct {
		name       string
		status     int
		retryable  bool
		statusCode string
	}{
		{name: "client failure", status: http.StatusBadRequest, retryable: false, statusCode: "4xx"},
		{name: "rate limited", status: http.StatusTooManyRequests, retryable: true, statusCode: "4xx"},
		{name: "server failure", status: http.StatusBadGateway, retryable: true, statusCode: "5xx"},
		{name: "unknown status", status: 99, retryable: false, statusCode: "unknown"},
	}
	for _, tt := range statusTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deliverer, err := NewDeliverer(DelivererConfig{
				Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader("secret response body"))}, nil
				}),
			})
			if err != nil {
				t.Fatalf("NewDeliverer() error = %v", err)
			}
			result, err := deliverer.Deliver(context.Background(), Endpoint{
				ID:            "we_1",
				TenantID:      "org_1",
				URL:           "https://example.com/hooks",
				SigningSecret: []byte("super-secret"),
				Events:        []string{"widget.created"},
			}, Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}, 1)
			if !errors.Is(err, ErrDeliveryFailed) || result.Retryable != tt.retryable || result.StatusClass != tt.statusCode {
				t.Fatalf("Deliver() result=%#v err=%v", result, err)
			}
			if strings.Contains(result.Error, "secret response body") {
				t.Fatalf("response body leaked in result: %#v", result)
			}
		})
	}
}

func TestDelivererReturnsSafeFailureForNon2xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"secret":"do-not-leak"}`))
	}))
	defer server.Close()

	deliverer, err := NewDeliverer(DelivererConfig{
		Client:         server.Client(),
		Clock:          fixedClock(),
		EndpointPolicy: EndpointPolicy{AllowInsecureHTTP: true},
	})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	result, err := deliverer.Deliver(context.Background(), Endpoint{
		ID:            "we_1",
		TenantID:      "org_1",
		URL:           server.URL,
		SigningSecret: []byte("super-secret"),
		Events:        []string{"widget.created"},
	}, Event{
		ID:       "evt_1",
		TenantID: "org_1",
		Type:     "widget.created",
		Payload:  json.RawMessage(`{"api_key":"should-not-leak"}`),
	}, 1)
	if !errors.Is(err, ErrDeliveryFailed) {
		t.Fatalf("Deliver() error = %v, want %v", err, ErrDeliveryFailed)
	}
	if result.Accepted || !result.Retryable || result.StatusClass != "5xx" {
		t.Fatalf("Deliver() result = %#v", result)
	}
	if strings.Contains(result.Error, "do-not-leak") || strings.Contains(result.Error, "should-not-leak") || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("unsafe failure leaked sensitive content: result=%#v err=%v", result, err)
	}
}

func TestDelivererValidationFailuresAreSafe(t *testing.T) {
	t.Parallel()

	result, err := (*Deliverer)(nil).Deliver(context.Background(), Endpoint{}, Event{}, 1)
	if !errors.Is(err, ErrDeliveryFailed) || result.Error != "webhook deliverer not configured" {
		t.Fatalf("nil Deliver() result=%#v err=%v", result, err)
	}
	deliverer, err := NewDeliverer(DelivererConfig{Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected request")
		return nil, errors.New("unexpected request")
	})})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	result, err = deliverer.Deliver(context.Background(), Endpoint{ID: "we_1", TenantID: "org_2", URL: "https://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}, Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"secret":"payload"}`)}, 1)
	if !errors.Is(err, ErrTenantMismatch) || strings.Contains(result.Error, "payload") {
		t.Fatalf("tenant mismatch result=%#v err=%v", result, err)
	}
	result, err = deliverer.Deliver(context.Background(), Endpoint{ID: "we_1", TenantID: "org_1", URL: "ftp://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}, Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}, 1)
	if !errors.Is(err, ErrInvalidEndpoint) || result.Error != "webhook endpoint invalid" {
		t.Fatalf("invalid endpoint result=%#v err=%v", result, err)
	}
	result, err = deliverer.Deliver(context.Background(), Endpoint{ID: "we_1", TenantID: "org_1", URL: "https://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}, Event{ID: "", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}, 1)
	if !errors.Is(err, ErrInvalidEvent) || result.Error != "webhook event invalid" {
		t.Fatalf("invalid event result=%#v err=%v", result, err)
	}
}

func TestAsyncHandlerLoadsEndpointAndRecordsAttempt(t *testing.T) {
	t.Parallel()

	var delivered bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	endpoints := &fakeGetter{endpoint: Endpoint{
		ID:            "we_1",
		TenantID:      "org_1",
		URL:           server.URL,
		SigningSecret: []byte("super-secret"),
		Events:        []string{"widget.created"},
	}}
	recorder := &fakeAttemptRecorder{}
	deliverer, err := NewDeliverer(DelivererConfig{
		Client:         server.Client(),
		Clock:          fixedClock(),
		EndpointPolicy: EndpointPolicy{AllowInsecureHTTP: true},
	})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	handler, err := NewHandler(HandlerConfig{Endpoints: endpoints, Deliverer: deliverer, Attempts: recorder})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	payload, err := EncodeJobPayload(JobPayload{
		DeliveryID: "del_1",
		EndpointID: "we_1",
		Event: Event{
			ID:       "evt_1",
			TenantID: "org_1",
			Type:     "widget.created",
			Payload:  json.RawMessage(`{"id":"w_1"}`),
		},
	})
	if err != nil {
		t.Fatalf("EncodeJobPayload() error = %v", err)
	}

	if err := handler.Handle(context.Background(), async.Job{ID: "del_1", Kind: "widget.created", TenantID: "org_1", Payload: payload, Attempts: 1}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !delivered {
		t.Fatal("webhook was not delivered")
	}
	if len(recorder.results) != 1 || !recorder.results[0].Accepted || recorder.results[0].DeliveryID != "del_1" {
		t.Fatalf("recorded attempts = %#v", recorder.results)
	}
}

func TestAsyncHandlerErrorPaths(t *testing.T) {
	t.Parallel()

	if _, err := NewHandler(HandlerConfig{}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("NewHandler() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	if err := (*Handler)(nil).Handle(context.Background(), async.Job{}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("nil Handle() error = %v, want %v", err, ErrStoreNotConfigured)
	}
	deliverer, err := NewDeliverer(DelivererConfig{Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	handler, err := NewHandler(HandlerConfig{Endpoints: &fakeGetter{}, Deliverer: deliverer})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	if err := handler.Handle(context.Background(), async.Job{ID: "del_1", TenantID: "org_1", Payload: []byte(`not json`)}); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("invalid payload error = %v, want %v", err, ErrInvalidDelivery)
	}
	payload, err := EncodeJobPayload(JobPayload{DeliveryID: "del_1", EndpointID: "we_1", Event: Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)}})
	if err != nil {
		t.Fatalf("EncodeJobPayload() error = %v", err)
	}
	if err := handler.Handle(context.Background(), async.Job{ID: "other", TenantID: "org_1", Payload: payload}); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("job id mismatch error = %v, want %v", err, ErrInvalidDelivery)
	}
	if err := handler.Handle(context.Background(), async.Job{ID: "del_1", TenantID: "org_1", Payload: payload}); !errors.Is(err, ErrDeliveryNotFound) {
		t.Fatalf("missing endpoint error = %v, want %v", err, ErrDeliveryNotFound)
	}

	recorder := &fakeAttemptRecorder{err: errors.New("record failed")}
	handler, err = NewHandler(HandlerConfig{
		Endpoints: &fakeGetter{endpoint: Endpoint{ID: "we_1", TenantID: "org_1", URL: "https://example.com/hooks", SigningSecret: []byte("secret"), Events: []string{"widget.created"}}},
		Deliverer: deliverer,
		Attempts:  recorder,
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	err = handler.Handle(context.Background(), async.Job{ID: "del_1", TenantID: "org_1", Payload: payload})
	if err == nil || !strings.Contains(err.Error(), "record failed") {
		t.Fatalf("record failure error = %v", err)
	}
}

func TestAsyncHandlerRejectsTenantMismatch(t *testing.T) {
	t.Parallel()

	deliverer, err := NewDeliverer(DelivererConfig{Client: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("unexpected delivery")
		return nil, errors.New("unexpected delivery")
	})})
	if err != nil {
		t.Fatalf("NewDeliverer() error = %v", err)
	}
	handler, err := NewHandler(HandlerConfig{Endpoints: &fakeGetter{}, Deliverer: deliverer})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	payload, err := EncodeJobPayload(JobPayload{
		DeliveryID: "del_1",
		EndpointID: "we_1",
		Event:      Event{ID: "evt_1", TenantID: "org_2", Type: "widget.created", Payload: json.RawMessage(`{"id":"w_1"}`)},
	})
	if err != nil {
		t.Fatalf("EncodeJobPayload() error = %v", err)
	}
	if err := handler.Handle(context.Background(), async.Job{ID: "del_1", TenantID: "org_1", Payload: payload}); !errors.Is(err, ErrTenantMismatch) {
		t.Fatalf("Handle() error = %v, want %v", err, ErrTenantMismatch)
	}
}

func TestJobPayloadValidationAndReplayErrors(t *testing.T) {
	t.Parallel()

	if _, err := EncodeJobPayload(JobPayload{}); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("EncodeJobPayload() error = %v, want %v", err, ErrInvalidDelivery)
	}
	if _, err := EncodeJobPayload(JobPayload{DeliveryID: "del_1", EndpointID: "we_1", Event: Event{ID: "evt_1", TenantID: "org_1", Type: "bad event", Payload: json.RawMessage(`{"id":"w_1"}`)}}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("EncodeJobPayload() invalid event error = %v, want %v", err, ErrInvalidEvent)
	}
	for _, payload := range [][]byte{nil, []byte(`not json`), []byte(`{"delivery_id":"","endpoint_id":"we_1"}`)} {
		if _, err := DecodeJobPayload(payload); !errors.Is(err, ErrInvalidDelivery) {
			t.Fatalf("DecodeJobPayload(%q) error = %v, want %v", payload, err, ErrInvalidDelivery)
		}
	}
	if err := Replay(context.Background(), nil, ReplayCommand{TenantID: "org_1", DeliveryID: "del_1"}); !errors.Is(err, ErrStoreNotConfigured) {
		t.Fatalf("Replay(nil) error = %v, want %v", err, ErrStoreNotConfigured)
	}
}

func TestReplayValidatesTenantScopedCommand(t *testing.T) {
	t.Parallel()

	store := &fakeReplayer{}
	when := fixedClock()()
	if err := Replay(context.Background(), store, ReplayCommand{TenantID: "org_1", DeliveryID: "del_1", NextAt: when}); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if store.tenantID != "org_1" || store.deliveryID != "del_1" || !store.nextAt.Equal(when) {
		t.Fatalf("replay call = %#v", store)
	}
	if err := Replay(context.Background(), store, ReplayCommand{TenantID: "", DeliveryID: "del_1"}); !errors.Is(err, ErrInvalidDelivery) {
		t.Fatalf("Replay() error = %v, want %v", err, ErrInvalidDelivery)
	}
}

func TestBackoffIsBounded(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{BaseDelay: time.Second, MaxDelay: 10 * time.Second}
	if got := policy.NextDelay(1); got != time.Second {
		t.Fatalf("NextDelay(1) = %s, want 1s", got)
	}
	if got := policy.NextDelay(5); got != 10*time.Second {
		t.Fatalf("NextDelay(5) = %s, want 10s cap", got)
	}
}

func withEndpointURL(endpoint Endpoint, url string) Endpoint {
	endpoint.URL = url
	return endpoint
}

func withEndpointSecret(endpoint Endpoint, secret []byte) Endpoint {
	endpoint.SigningSecret = secret
	return endpoint
}

func withEndpointHeaders(endpoint Endpoint, headers http.Header) Endpoint {
	endpoint.Headers = headers
	return endpoint
}

func withEndpointEvents(endpoint Endpoint, events []string) Endpoint {
	endpoint.Events = events
	return endpoint
}

func fixedClock() func() time.Time {
	return func() time.Time {
		return time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	}
}

type fakeRegistry struct {
	endpoints []Endpoint
	err       error
}

func (f *fakeRegistry) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]Endpoint, error) {
	_ = ctx
	if f.err != nil {
		return nil, f.err
	}
	var out []Endpoint
	for _, endpoint := range f.endpoints {
		if endpoint.TenantID == tenantID && endpoint.SubscribedTo(eventType) {
			out = append(out, endpoint)
		}
	}
	return out, nil
}

type enqueuedDelivery struct {
	delivery Delivery
	job      JobPayload
}

type fakeDeliveryStore struct {
	enqueued []enqueuedDelivery
	err      error
}

func (f *fakeDeliveryStore) EnqueueDelivery(ctx context.Context, delivery Delivery, job JobPayload) error {
	_ = ctx
	if f.err != nil {
		return f.err
	}
	f.enqueued = append(f.enqueued, enqueuedDelivery{delivery: delivery, job: job})
	return nil
}

type fakeGetter struct {
	endpoint Endpoint
	found    bool
}

func (f *fakeGetter) GetEndpoint(ctx context.Context, tenantID, endpointID string) (Endpoint, bool, error) {
	_ = ctx
	if f.found || (f.endpoint.TenantID == tenantID && f.endpoint.ID == endpointID) {
		return f.endpoint, true, nil
	}
	return Endpoint{}, false, nil
}

type fakeAttemptRecorder struct {
	results []AttemptResult
	err     error
}

func (f *fakeAttemptRecorder) RecordAttempt(ctx context.Context, result AttemptResult) error {
	_ = ctx
	if f.err != nil {
		return f.err
	}
	f.results = append(f.results, result)
	return nil
}

type fakeReplayer struct {
	tenantID   string
	deliveryID string
	nextAt     time.Time
}

func (f *fakeReplayer) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	_ = ctx
	f.tenantID = tenantID
	f.deliveryID = deliveryID
	f.nextAt = nextAt
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}
