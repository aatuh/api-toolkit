package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"

	"example.com/reference-saas-api/internal/domain"
)

var webhookEventTypes = []string{
	"widget.created",
	"widget.updated",
	"widget.deleted",
	"widget.import.completed",
	// api-toolkit:webhook-event-types
}

type WebhookService struct {
	mu             sync.Mutex
	nextEndpoint   int
	nextEvent      int
	tenancy        *TenancyService
	now            func() time.Time
	newEndpointID  func() (string, error)
	newEventID     func() (string, error)
	newSecret      func() (string, error)
	catalog        webhookdelivery.Catalog
	endpointPolicy webhookdelivery.EndpointPolicy
	endpoints      map[string]endpointRecord
	deliveries     map[string]webhookdelivery.Delivery
	jobs           map[string]webhookdelivery.JobPayload
	store          WebhookStore
}

type endpointRecord struct {
	endpoint   webhookdelivery.Endpoint
	secretHash string
}

type WebhookEndpointCreated struct {
	Endpoint webhookdelivery.Endpoint
	Secret   string
}

type WebhookStore interface {
	CreateEndpoint(ctx context.Context, endpoint webhookdelivery.Endpoint) error
	ListEndpointsForActor(ctx context.Context, tenantID string) ([]webhookdelivery.Endpoint, error)
	ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error)
	GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error)
	EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error
	RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error
	ListDeliveries(ctx context.Context, tenantID string) ([]webhookdelivery.Delivery, error)
	GetDelivery(ctx context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error)
	ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error
}

func NewWebhookService(tenancy *TenancyService) *WebhookService {
	return NewWebhookServiceWithEndpointPolicy(tenancy, webhookdelivery.EndpointPolicy{})
}

func NewWebhookServiceWithEndpointPolicy(tenancy *TenancyService, endpointPolicy webhookdelivery.EndpointPolicy) *WebhookService {
	catalog, _ := webhookdelivery.NewCatalog(webhookEventTypes...)
	return &WebhookService{
		tenancy:        tenancy,
		now:            time.Now,
		newEndpointID:  func() (string, error) { return randomPrefixedID("whend") },
		newEventID:     func() (string, error) { return randomPrefixedID("evt") },
		newSecret:      randomToken,
		catalog:        catalog,
		endpointPolicy: endpointPolicy,
		endpoints:      map[string]endpointRecord{},
		deliveries:     map[string]webhookdelivery.Delivery{},
		jobs:           map[string]webhookdelivery.JobPayload{},
	}
}

func NewWebhookServiceWithStore(tenancy *TenancyService, store WebhookStore) *WebhookService {
	return NewWebhookServiceWithStoreAndEndpointPolicy(tenancy, store, webhookdelivery.EndpointPolicy{})
}

func NewWebhookServiceWithStoreAndEndpointPolicy(tenancy *TenancyService, store WebhookStore, endpointPolicy webhookdelivery.EndpointPolicy) *WebhookService {
	service := NewWebhookServiceWithEndpointPolicy(tenancy, endpointPolicy)
	service.store = store
	return service
}

func (s *WebhookService) EventTypes() []string {
	out := append([]string(nil), webhookEventTypes...)
	sort.Strings(out)
	return out
}

func (s *WebhookService) CreateEndpoint(ctx context.Context, actorID, tenantID, targetURL string, events []string) (WebhookEndpointCreated, error) {
	if err := ctx.Err(); err != nil {
		return WebhookEndpointCreated{}, err
	}
	if s == nil || s.tenancy == nil {
		return WebhookEndpointCreated{}, ErrValidation
	}
	actorID = strings.TrimSpace(actorID)
	tenantID = strings.TrimSpace(tenantID)
	targetURL = strings.TrimSpace(targetURL)
	if actorID == "" || tenantID == "" || targetURL == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleAdmin)
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	if !ok {
		return WebhookEndpointCreated{}, ErrForbidden
	}
	cleanEvents, err := s.cleanEndpointEvents(events)
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	secret, err := s.newSecret()
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}
	endpointID, err := s.newEndpointID()
	if err != nil {
		return WebhookEndpointCreated{}, err
	}
	endpointID = strings.TrimSpace(endpointID)
	if endpointID == "" {
		return WebhookEndpointCreated{}, ErrValidation
	}

	now := s.now().UTC()
	endpoint := webhookdelivery.Endpoint{
		ID:            endpointID,
		TenantID:      tenantID,
		URL:           targetURL,
		SigningSecret: []byte(secret),
		Events:        cleanEvents,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := webhookdelivery.ValidateEndpoint(endpoint, s.endpointPolicy); err != nil {
		return WebhookEndpointCreated{}, ErrValidation
	}
	if s.store != nil {
		if err := s.store.CreateEndpoint(ctx, endpoint); err != nil {
			return WebhookEndpointCreated{}, err
		}
		return WebhookEndpointCreated{Endpoint: publicWebhookEndpoint(endpoint), Secret: secret}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[endpoint.ID] = endpointRecord{endpoint: endpoint, secretHash: hashWebhookSecret(secret)}
	return WebhookEndpointCreated{Endpoint: publicWebhookEndpoint(endpoint), Secret: secret}, nil
}

func (s *WebhookService) ListEndpointsForActor(ctx context.Context, actorID, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if s.store != nil {
		endpoints, err := s.store.ListEndpointsForActor(ctx, strings.TrimSpace(tenantID))
		if err != nil {
			return nil, err
		}
		out := make([]webhookdelivery.Endpoint, 0, len(endpoints))
		for _, endpoint := range endpoints {
			out = append(out, publicWebhookEndpoint(endpoint))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Endpoint, 0)
	for _, record := range s.endpoints {
		if strings.TrimSpace(record.endpoint.TenantID) == strings.TrimSpace(tenantID) {
			out = append(out, publicWebhookEndpoint(record.endpoint))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || !s.catalog.Allows(eventType) {
		return nil, ErrValidation
	}
	if s.store != nil {
		return s.store.ListEndpoints(ctx, tenantID, eventType)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Endpoint, 0)
	for _, record := range s.endpoints {
		endpoint := record.endpoint
		if strings.TrimSpace(endpoint.TenantID) == tenantID && endpoint.SubscribedTo(eventType) {
			out = append(out, cloneWebhookEndpoint(endpoint))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Endpoint{}, false, err
	}
	if s == nil {
		return webhookdelivery.Endpoint{}, false, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	endpointID = strings.TrimSpace(endpointID)
	if tenantID == "" || endpointID == "" {
		return webhookdelivery.Endpoint{}, false, ErrValidation
	}
	if s.store != nil {
		return s.store.GetEndpoint(ctx, tenantID, endpointID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.endpoints[endpointID]
	if !ok || strings.TrimSpace(record.endpoint.TenantID) != tenantID {
		return webhookdelivery.Endpoint{}, false, nil
	}
	return cloneWebhookEndpoint(record.endpoint), true, nil
}

func (s *WebhookService) EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	if err := webhookdelivery.ValidateDelivery(delivery); err != nil {
		return ErrValidation
	}
	if job.Event.TenantID != delivery.TenantID || job.DeliveryID != delivery.ID || job.EndpointID != delivery.EndpointID {
		return ErrValidation
	}
	if s.store != nil {
		return s.store.EnqueueDelivery(ctx, delivery, job)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.deliveries[delivery.ID]; exists {
		return nil
	}
	s.deliveries[delivery.ID] = delivery
	s.jobs[delivery.ID] = job
	return nil
}

func (s *WebhookService) RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	result.DeliveryID = strings.TrimSpace(result.DeliveryID)
	result.TenantID = strings.TrimSpace(result.TenantID)
	result.EndpointID = strings.TrimSpace(result.EndpointID)
	if result.DeliveryID == "" || result.TenantID == "" || result.EndpointID == "" || result.Attempt <= 0 {
		return ErrValidation
	}
	if s.store != nil {
		return s.store.RecordAttempt(ctx, result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[result.DeliveryID]
	if !ok || strings.TrimSpace(delivery.TenantID) != result.TenantID || strings.TrimSpace(delivery.EndpointID) != result.EndpointID {
		return ErrNotFound
	}
	delivery.Attempt = result.Attempt
	delivery.LastStatusCode = result.StatusCode
	delivery.LastError = safeWebhookAttemptError(result.Error)
	delivery.State = webhookdelivery.StateFailed
	if result.Accepted {
		delivery.State = webhookdelivery.StateSucceeded
		delivery.LastError = ""
	} else if !result.Retryable {
		delivery.State = webhookdelivery.StateDeadLetter
	}
	if result.OccurredAt.IsZero() {
		delivery.UpdatedAt = s.now().UTC()
	} else {
		delivery.UpdatedAt = result.OccurredAt.UTC()
	}
	s.deliveries[result.DeliveryID] = delivery
	return nil
}

func (s *WebhookService) DispatchEvent(ctx context.Context, tenantID, eventType string, payload any) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || !s.catalog.Allows(eventType) {
		return nil, ErrValidation
	}
	encoded, err := json.Marshal(payload)
	if err != nil || !json.Valid(encoded) {
		return nil, ErrValidation
	}
	dispatcher, err := webhookdelivery.NewDispatcher(webhookdelivery.DispatcherConfig{
		Catalog:        s.catalog,
		Endpoints:      s,
		Store:          s,
		Clock:          s.now,
		EndpointPolicy: s.endpointPolicy,
	})
	if err != nil {
		return nil, err
	}
	eventID, err := s.nextWebhookEventID()
	if err != nil {
		return nil, err
	}
	return dispatcher.Dispatch(ctx, webhookdelivery.Event{
		ID:       eventID,
		TenantID: tenantID,
		Type:     eventType,
		Payload:  encoded,
	})
}

func (s *WebhookService) ListDeliveriesForActor(ctx context.Context, actorID, tenantID string) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	if s.store != nil {
		return s.store.ListDeliveries(ctx, strings.TrimSpace(tenantID))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]webhookdelivery.Delivery, 0)
	for _, delivery := range s.deliveries {
		if strings.TrimSpace(delivery.TenantID) == strings.TrimSpace(tenantID) {
			out = append(out, delivery)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *WebhookService) ReplayDeliveryForActor(ctx context.Context, actorID, tenantID, deliveryID string) (webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if s == nil || s.tenancy == nil {
		return webhookdelivery.Delivery{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleAdmin)
	if err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if !ok {
		return webhookdelivery.Delivery{}, ErrForbidden
	}
	if err := s.ReplayDelivery(ctx, tenantID, deliveryID, s.now().UTC()); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	if s.store != nil {
		delivery, ok, err := s.store.GetDelivery(ctx, tenantID, deliveryID)
		if err != nil {
			return webhookdelivery.Delivery{}, err
		}
		if !ok {
			return webhookdelivery.Delivery{}, ErrNotFound
		}
		return delivery, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveries[strings.TrimSpace(deliveryID)], nil
}

func (s *WebhookService) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return ErrValidation
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return ErrValidation
	}
	if s.store != nil {
		if err := s.store.ReplayDelivery(ctx, tenantID, deliveryID, nextAt); err != nil {
			if errors.Is(err, webhookdelivery.ErrDeliveryNotFound) {
				return ErrNotFound
			}
			return err
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery, ok := s.deliveries[deliveryID]
	if !ok || strings.TrimSpace(delivery.TenantID) != tenantID {
		return ErrNotFound
	}
	if nextAt.IsZero() {
		nextAt = s.now().UTC()
	}
	delivery.State = webhookdelivery.StatePending
	delivery.NextAt = nextAt.UTC()
	delivery.UpdatedAt = s.now().UTC()
	s.deliveries[deliveryID] = delivery
	return nil
}

func (s *WebhookService) cleanEndpointEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, ErrValidation
	}
	cleaned := make([]string, 0, len(events))
	seen := map[string]struct{}{}
	for _, eventType := range events {
		eventType = strings.TrimSpace(eventType)
		if eventType == webhookdelivery.AnyEventType {
			if _, ok := seen[eventType]; !ok {
				cleaned = append(cleaned, eventType)
				seen[eventType] = struct{}{}
			}
			continue
		}
		if !s.catalog.Allows(eventType) {
			return nil, ErrValidation
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		cleaned = append(cleaned, eventType)
		seen[eventType] = struct{}{}
	}
	if len(cleaned) == 0 {
		return nil, ErrValidation
	}
	sort.Strings(cleaned)
	return cleaned, nil
}

func (s *WebhookService) nextWebhookEventID() (string, error) {
	if s != nil && s.newEventID != nil {
		id, err := s.newEventID()
		if err != nil {
			return "", err
		}
		id = strings.TrimSpace(id)
		if id == "" {
			return "", ErrValidation
		}
		return id, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextEvent++
	return fmt.Sprintf("evt_%06d", s.nextEvent), nil
}

func publicWebhookEndpoint(endpoint webhookdelivery.Endpoint) webhookdelivery.Endpoint {
	endpoint = cloneWebhookEndpoint(endpoint)
	endpoint.SigningSecret = nil
	return endpoint
}

func cloneWebhookEndpoint(endpoint webhookdelivery.Endpoint) webhookdelivery.Endpoint {
	endpoint.SigningSecret = append([]byte(nil), endpoint.SigningSecret...)
	endpoint.Events = append([]string(nil), endpoint.Events...)
	if endpoint.Headers != nil {
		endpoint.Headers = endpoint.Headers.Clone()
	}
	return endpoint
}

func hashWebhookSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func safeWebhookAttemptError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") || strings.Contains(lower, "apikey") {
		return "delivery failed"
	}
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

var _ webhookdelivery.EndpointRegistry = (*WebhookService)(nil)
var _ webhookdelivery.EndpointGetter = (*WebhookService)(nil)
var _ webhookdelivery.DeliveryEnqueuer = (*WebhookService)(nil)
var _ webhookdelivery.AttemptRecorder = (*WebhookService)(nil)
var _ webhookdelivery.Replayer = (*WebhookService)(nil)
