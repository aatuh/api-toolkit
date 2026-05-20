package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"
)

func TestWebhookServiceCreatesEndpointAndDispatchesTenantDelivery(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks/widgets", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if created.Secret != "webhook-secret-value" || created.Endpoint.ID == "" || len(created.Endpoint.SigningSecret) != 0 {
		t.Fatalf("created endpoint leaked or missed secret data: %#v", created)
	}

	listed, err := service.ListEndpointsForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListEndpointsForActor() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.Endpoint.ID || len(listed[0].SigningSecret) != 0 {
		t.Fatalf("listed endpoints = %#v", listed)
	}

	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].EndpointID != created.Endpoint.ID || deliveries[0].State != webhookdelivery.StatePending {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	deliveryList, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	encoded, err := json.Marshal(deliveryList)
	if err != nil {
		t.Fatalf("marshal deliveries: %v", err)
	}
	if strings.Contains(string(encoded), created.Secret) {
		t.Fatalf("delivery list leaked signing secret: %s", encoded)
	}
	if _, ok, err := service.GetEndpoint(ctx, org.ID, created.Endpoint.ID); err != nil || !ok {
		t.Fatalf("GetEndpoint() ok=%v err=%v", ok, err)
	}
	if _, ok, err := service.GetEndpoint(ctx, "org_other", created.Endpoint.ID); err != nil || ok {
		t.Fatalf("cross-tenant GetEndpoint() ok=%v err=%v", ok, err)
	}
}

func TestWebhookServiceRejectsUnsafeEndpointAndReplaysTenantScopedDelivery(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }

	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "http://example.com/insecure", []string{"widget.created"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsafe endpoint error = %v, want %v", err, ErrValidation)
	}
	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"unsupported.event"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsupported event error = %v, want %v", err, ErrValidation)
	}

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	if _, err := service.ReplayDeliveryForActor(ctx, "owner_1", "org_other", deliveries[0].ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-tenant replay error = %v, want %v", err, ErrForbidden)
	}
	replayed, err := service.ReplayDeliveryForActor(ctx, "owner_1", org.ID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("ReplayDeliveryForActor() error = %v", err)
	}
	if replayed.ID != deliveries[0].ID || replayed.EndpointID != created.Endpoint.ID || replayed.State != webhookdelivery.StatePending {
		t.Fatalf("replayed delivery = %#v", replayed)
	}
}

func TestWebhookServiceAllowsInsecureEndpointOnlyWhenConfigured(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookServiceWithEndpointPolicy(tenancy, webhookdelivery.EndpointPolicy{AllowInsecureHTTP: true})
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }
	if _, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "http://127.0.0.1:18081/webhooks", []string{"widget.created"}); err != nil {
		t.Fatalf("CreateEndpoint() with development HTTP policy error = %v", err)
	}
}

func TestWebhookServiceRecordsAttemptsWithoutSecretLeak(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewWebhookService(tenancy)
	service.newSecret = func() (string, error) { return "webhook-secret-value", nil }
	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %#v", deliveries)
	}
	result := webhookdelivery.AttemptResult{
		DeliveryID: deliveries[0].ID,
		TenantID:   org.ID,
		EndpointID: created.Endpoint.ID,
		EventID:    deliveries[0].EventID,
		EventType:  "widget.created",
		Attempt:    1,
		Retryable:  true,
		Error:      "dial webhook-secret-value failed",
		OccurredAt: time.Unix(1_700_000_100, 0).UTC(),
	}
	if err := service.RecordAttempt(ctx, result); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}
	listed, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != webhookdelivery.StateFailed || listed[0].LastError != "delivery failed" {
		t.Fatalf("listed delivery after attempt = %#v", listed)
	}
	encoded, err := json.Marshal(listed)
	if err != nil {
		t.Fatalf("marshal deliveries: %v", err)
	}
	if strings.Contains(string(encoded), "webhook-secret-value") {
		t.Fatalf("attempt result leaked signing secret: %s", encoded)
	}
}

func TestWebhookServiceWithStorePersistsEndpointsAndDeliveries(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	store := newRecordingWebhookStore()
	service := NewWebhookServiceWithStore(tenancy, store)
	service.newEndpointID = func() (string, error) { return "whend_store", nil }
	service.newEventID = func() (string, error) { return "evt_store", nil }
	service.newSecret = func() (string, error) { return "stored-webhook-secret", nil }

	created, err := service.CreateEndpoint(ctx, "owner_1", org.ID, "https://example.com/webhooks", []string{"widget.created"})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if created.Endpoint.ID != "whend_store" || len(created.Endpoint.SigningSecret) != 0 || created.Secret != "stored-webhook-secret" {
		t.Fatalf("created endpoint = %#v", created)
	}
	if string(store.endpoint.SigningSecret) != "stored-webhook-secret" {
		t.Fatalf("store did not receive signing secret")
	}

	listed, err := service.ListEndpointsForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListEndpointsForActor() error = %v", err)
	}
	if len(listed) != 1 || len(listed[0].SigningSecret) != 0 {
		t.Fatalf("listed endpoints leaked secret: %#v", listed)
	}

	deliveries, err := service.DispatchEvent(ctx, org.ID, "widget.created", map[string]any{"id": "wgt_1"})
	if err != nil {
		t.Fatalf("DispatchEvent() error = %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ID == "" || store.job.DeliveryID != deliveries[0].ID {
		t.Fatalf("deliveries=%#v job=%#v", deliveries, store.job)
	}
	listedDeliveries, err := service.ListDeliveriesForActor(ctx, "owner_1", org.ID)
	if err != nil {
		t.Fatalf("ListDeliveriesForActor() error = %v", err)
	}
	if len(listedDeliveries) != 1 || listedDeliveries[0].TenantID != org.ID {
		t.Fatalf("listed deliveries = %#v", listedDeliveries)
	}
	replayed, err := service.ReplayDeliveryForActor(ctx, "owner_1", org.ID, deliveries[0].ID)
	if err != nil {
		t.Fatalf("ReplayDeliveryForActor() error = %v", err)
	}
	if replayed.ID != deliveries[0].ID || replayed.State != webhookdelivery.StatePending {
		t.Fatalf("replayed delivery = %#v", replayed)
	}
}

type recordingWebhookStore struct {
	endpoint   webhookdelivery.Endpoint
	delivery   webhookdelivery.Delivery
	job        webhookdelivery.JobPayload
	replayedID string
}

func newRecordingWebhookStore() *recordingWebhookStore {
	return &recordingWebhookStore{}
}

func (s *recordingWebhookStore) CreateEndpoint(_ context.Context, endpoint webhookdelivery.Endpoint) error {
	s.endpoint = cloneWebhookEndpoint(endpoint)
	return nil
}

func (s *recordingWebhookStore) ListEndpointsForActor(_ context.Context, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) {
		return nil, nil
	}
	return []webhookdelivery.Endpoint{cloneWebhookEndpoint(s.endpoint)}, nil
}

func (s *recordingWebhookStore) ListEndpoints(_ context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) || !s.endpoint.SubscribedTo(eventType) {
		return nil, nil
	}
	return []webhookdelivery.Endpoint{cloneWebhookEndpoint(s.endpoint)}, nil
}

func (s *recordingWebhookStore) GetEndpoint(_ context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if strings.TrimSpace(s.endpoint.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.endpoint.ID) != strings.TrimSpace(endpointID) {
		return webhookdelivery.Endpoint{}, false, nil
	}
	return cloneWebhookEndpoint(s.endpoint), true, nil
}

func (s *recordingWebhookStore) EnqueueDelivery(_ context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	s.delivery = delivery
	s.job = job
	return nil
}

func (s *recordingWebhookStore) RecordAttempt(_ context.Context, result webhookdelivery.AttemptResult) error {
	if strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(result.DeliveryID) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	if result.Accepted {
		s.delivery.State = webhookdelivery.StateSucceeded
	} else if result.Retryable {
		s.delivery.State = webhookdelivery.StateFailed
	} else {
		s.delivery.State = webhookdelivery.StateDeadLetter
	}
	s.delivery.Attempt = result.Attempt
	s.delivery.LastStatusCode = result.StatusCode
	s.delivery.LastError = result.Error
	return nil
}

func (s *recordingWebhookStore) ListDeliveries(_ context.Context, tenantID string) ([]webhookdelivery.Delivery, error) {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) {
		return nil, nil
	}
	return []webhookdelivery.Delivery{s.delivery}, nil
}

func (s *recordingWebhookStore) GetDelivery(_ context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error) {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(deliveryID) {
		return webhookdelivery.Delivery{}, false, nil
	}
	return s.delivery, true, nil
}

func (s *recordingWebhookStore) ReplayDelivery(_ context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if strings.TrimSpace(s.delivery.TenantID) != strings.TrimSpace(tenantID) || strings.TrimSpace(s.delivery.ID) != strings.TrimSpace(deliveryID) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	s.replayedID = strings.TrimSpace(deliveryID)
	s.delivery.State = webhookdelivery.StatePending
	s.delivery.NextAt = nextAt.UTC()
	return nil
}
