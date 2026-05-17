package webhookdelivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v3/async"
	"github.com/aatuh/api-toolkit/v3/webhooks"
)

const (
	// AnyEventType subscribes an endpoint to every event type in the catalog.
	AnyEventType = "*"

	defaultHTTPTimeout = 10 * time.Second
	defaultBaseDelay   = time.Second
	defaultMaxDelay    = time.Minute
	maxLabelLength     = 64
)

var (
	// ErrInvalidEndpoint reports a webhook endpoint with missing or unsafe fields.
	ErrInvalidEndpoint = errors.New("invalid webhook endpoint")
	// ErrInvalidEvent reports a webhook event with missing or malformed fields.
	ErrInvalidEvent = errors.New("invalid webhook event")
	// ErrUnsupportedEvent reports an event type not present in the event catalog.
	ErrUnsupportedEvent = errors.New("unsupported webhook event")
	// ErrUnsafeHeader reports endpoint headers that could carry secrets or break signing.
	ErrUnsafeHeader = errors.New("unsafe webhook header")
	// ErrInvalidDelivery reports a malformed delivery or replay command.
	ErrInvalidDelivery = errors.New("invalid webhook delivery")
	// ErrTenantMismatch reports a job, event, endpoint, or delivery tenant mismatch.
	ErrTenantMismatch = errors.New("webhook tenant mismatch")
	// ErrDeliveryNotFound reports a missing tenant-scoped endpoint or delivery.
	ErrDeliveryNotFound = errors.New("webhook delivery not found")
	// ErrDeliveryFailed reports a delivery attempt that was not accepted by the receiver.
	ErrDeliveryFailed = errors.New("webhook delivery failed")
	// ErrStoreNotConfigured reports missing registry, store, or recorder dependencies.
	ErrStoreNotConfigured = errors.New("webhook delivery store not configured")
)

// Endpoint is a tenant-scoped outbound webhook target.
//
// SigningSecret is intentionally omitted from JSON so durable jobs and logs do
// not accidentally persist raw endpoint secrets.
type Endpoint struct {
	ID            string      `json:"id"`
	TenantID      string      `json:"tenant_id"`
	URL           string      `json:"url"`
	SigningSecret []byte      `json:"-"`
	Events        []string    `json:"events"`
	Headers       http.Header `json:"headers,omitempty"`
	Disabled      bool        `json:"disabled,omitempty"`
	CreatedAt     time.Time   `json:"created_at,omitempty"`
	UpdatedAt     time.Time   `json:"updated_at,omitempty"`
}

// EndpointPolicy configures endpoint validation. HTTPS is required by default;
// tests and local-only development can explicitly allow HTTP.
type EndpointPolicy struct {
	AllowInsecureHTTP bool
}

// Event is the durable outbound webhook event envelope used by dispatchers and
// async worker jobs. Payload must be valid JSON.
type Event struct {
	ID         string          `json:"id"`
	TenantID   string          `json:"tenant_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at,omitempty"`
}

// DeliveryState describes durable outbound webhook delivery state.
type DeliveryState string

const (
	// StatePending means a delivery is queued for the worker.
	StatePending DeliveryState = "pending"
	// StateLeased means a worker owns the current attempt.
	StateLeased DeliveryState = "leased"
	// StateSucceeded means the receiver accepted the delivery.
	StateSucceeded DeliveryState = "succeeded"
	// StateFailed means the delivery attempt failed but can be retried.
	StateFailed DeliveryState = "failed"
	// StateDeadLetter means retry policy has been exhausted.
	StateDeadLetter DeliveryState = "dead_letter"
)

// Delivery is a tenant-scoped durable delivery record.
type Delivery struct {
	ID             string        `json:"id"`
	TenantID       string        `json:"tenant_id"`
	EndpointID     string        `json:"endpoint_id"`
	EventID        string        `json:"event_id"`
	EventType      string        `json:"event_type"`
	URL            string        `json:"url"`
	State          DeliveryState `json:"state"`
	Attempt        int           `json:"attempt"`
	NextAt         time.Time     `json:"next_at"`
	LastStatusCode int           `json:"last_status_code,omitempty"`
	LastError      string        `json:"last_error,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// JobPayload is the safe async payload for a delivery worker. It does not carry
// endpoint secrets; workers load the tenant-scoped endpoint before signing.
type JobPayload struct {
	DeliveryID string `json:"delivery_id"`
	EndpointID string `json:"endpoint_id"`
	Event      Event  `json:"event"`
}

// AttemptResult describes one outbound delivery attempt.
type AttemptResult struct {
	DeliveryID  string    `json:"delivery_id"`
	TenantID    string    `json:"tenant_id"`
	EndpointID  string    `json:"endpoint_id"`
	EventID     string    `json:"event_id"`
	EventType   string    `json:"event_type"`
	Attempt     int       `json:"attempt"`
	StatusCode  int       `json:"status_code,omitempty"`
	StatusClass string    `json:"status_class"`
	Accepted    bool      `json:"accepted"`
	Retryable   bool      `json:"retryable"`
	Error       string    `json:"error,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// DeliveryObservation is a low-cardinality metrics/logging event.
type DeliveryObservation struct {
	EventType   string
	Outcome     string
	StatusClass string
}

// MetricsRecorder records low-cardinality delivery outcomes.
type MetricsRecorder interface {
	ObserveWebhookDelivery(ctx context.Context, event DeliveryObservation)
}

// MetricsRecorderFunc adapts a function to MetricsRecorder.
type MetricsRecorderFunc func(context.Context, DeliveryObservation)

// ObserveWebhookDelivery records an outbound webhook event.
func (f MetricsRecorderFunc) ObserveWebhookDelivery(ctx context.Context, event DeliveryObservation) {
	if f != nil {
		f(ctx, event)
	}
}

// EndpointRegistry lists tenant-scoped endpoints for an event type.
type EndpointRegistry interface {
	ListEndpoints(ctx context.Context, tenantID, eventType string) ([]Endpoint, error)
}

// EndpointGetter loads one tenant-scoped endpoint.
type EndpointGetter interface {
	GetEndpoint(ctx context.Context, tenantID, endpointID string) (Endpoint, bool, error)
}

// DeliveryEnqueuer creates a durable delivery and queues its async job.
type DeliveryEnqueuer interface {
	EnqueueDelivery(ctx context.Context, delivery Delivery, job JobPayload) error
}

// AttemptRecorder records a durable delivery attempt.
type AttemptRecorder interface {
	RecordAttempt(ctx context.Context, result AttemptResult) error
}

// Replayer moves a tenant-scoped delivery back into pending state.
type Replayer interface {
	ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error
}

// Catalog is the allow-list of outbound event types a service can publish.
type Catalog struct {
	eventTypes map[string]struct{}
}

// NewCatalog creates a fail-closed event catalog.
func NewCatalog(eventTypes ...string) (Catalog, error) {
	catalog := Catalog{eventTypes: make(map[string]struct{}, len(eventTypes))}
	for _, eventType := range eventTypes {
		eventType = strings.TrimSpace(eventType)
		if !validEventType(eventType, false) {
			return Catalog{}, ErrInvalidEvent
		}
		catalog.eventTypes[eventType] = struct{}{}
	}
	if len(catalog.eventTypes) == 0 {
		return Catalog{}, ErrUnsupportedEvent
	}
	return catalog, nil
}

// Allows reports whether the catalog includes eventType.
func (c Catalog) Allows(eventType string) bool {
	_, ok := c.eventTypes[strings.TrimSpace(eventType)]
	return ok
}

// ValidateEvent validates required event fields and catalog membership.
func (c Catalog) ValidateEvent(event Event) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}
	if !c.Allows(event.Type) {
		return ErrUnsupportedEvent
	}
	return nil
}

// ValidateEvent verifies a webhook event without applying catalog policy.
func ValidateEvent(event Event) error {
	if strings.TrimSpace(event.ID) == "" ||
		strings.TrimSpace(event.TenantID) == "" ||
		!validEventType(strings.TrimSpace(event.Type), false) ||
		len(event.Payload) == 0 ||
		!json.Valid(event.Payload) {
		return ErrInvalidEvent
	}
	return nil
}

// ValidateEndpoint verifies required endpoint fields, URL policy, subscriptions,
// and safe static headers.
func ValidateEndpoint(endpoint Endpoint, policy EndpointPolicy) error {
	if strings.TrimSpace(endpoint.ID) == "" ||
		strings.TrimSpace(endpoint.TenantID) == "" ||
		len(endpoint.SigningSecret) == 0 ||
		len(endpoint.Events) == 0 {
		return ErrInvalidEndpoint
	}
	if err := validateEndpointURL(endpoint.URL, policy); err != nil {
		return err
	}
	for _, eventType := range endpoint.Events {
		if !validEventType(strings.TrimSpace(eventType), true) {
			return ErrInvalidEvent
		}
	}
	if _, err := SafeHeaders(endpoint.Headers); err != nil {
		return err
	}
	return nil
}

// SafeHeaders validates and clones endpoint static headers. Secret-bearing,
// hop-by-hop, and signing headers are rejected.
func SafeHeaders(headers http.Header) (http.Header, error) {
	if headers == nil {
		return http.Header{}, nil
	}
	clone := make(http.Header, len(headers))
	for name, values := range headers {
		name = http.CanonicalHeaderKey(strings.TrimSpace(name))
		if name == "" || containsControl(name) || unsafeHeaderName(name) {
			return nil, ErrUnsafeHeader
		}
		for _, value := range values {
			if containsControl(value) {
				return nil, ErrUnsafeHeader
			}
			clone.Add(name, strings.TrimSpace(value))
		}
	}
	return clone, nil
}

// SubscribedTo reports whether the endpoint subscribes to eventType.
func (endpoint Endpoint) SubscribedTo(eventType string) bool {
	eventType = strings.TrimSpace(eventType)
	for _, subscribed := range endpoint.Events {
		subscribed = strings.TrimSpace(subscribed)
		if subscribed == AnyEventType || subscribed == eventType {
			return true
		}
	}
	return false
}

// EndpointMatches verifies tenant, disabled state, and event subscription.
func EndpointMatches(endpoint Endpoint, event Event) bool {
	return !endpoint.Disabled &&
		strings.TrimSpace(endpoint.TenantID) == strings.TrimSpace(event.TenantID) &&
		endpoint.SubscribedTo(event.Type)
}

// DefaultDeliveryID returns a deterministic idempotency-friendly delivery id for
// an event/endpoint pair.
func DefaultDeliveryID(event Event, endpoint Endpoint) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(event.TenantID),
		strings.TrimSpace(event.ID),
		strings.TrimSpace(endpoint.ID),
	}, "\x00")))
	return "whdel_" + hex.EncodeToString(sum[:16])
}

// DispatcherConfig configures a Dispatcher.
type DispatcherConfig struct {
	Catalog        Catalog
	Endpoints      EndpointRegistry
	Store          DeliveryEnqueuer
	Clock          func() time.Time
	DeliveryIDFunc func(Event, Endpoint) string
	EndpointPolicy EndpointPolicy
}

// Dispatcher creates durable deliveries for subscribed tenant endpoints.
type Dispatcher struct {
	catalog        Catalog
	endpoints      EndpointRegistry
	store          DeliveryEnqueuer
	clock          func() time.Time
	deliveryIDFunc func(Event, Endpoint) string
	endpointPolicy EndpointPolicy
}

// NewDispatcher constructs a Dispatcher.
func NewDispatcher(cfg DispatcherConfig) (*Dispatcher, error) {
	if cfg.Endpoints == nil || cfg.Store == nil {
		return nil, ErrStoreNotConfigured
	}
	if len(cfg.Catalog.eventTypes) == 0 {
		return nil, ErrUnsupportedEvent
	}
	clock := cfg.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	deliveryIDFunc := cfg.DeliveryIDFunc
	if deliveryIDFunc == nil {
		deliveryIDFunc = DefaultDeliveryID
	}
	return &Dispatcher{
		catalog:        cfg.Catalog,
		endpoints:      cfg.Endpoints,
		store:          cfg.Store,
		clock:          clock,
		deliveryIDFunc: deliveryIDFunc,
		endpointPolicy: cfg.EndpointPolicy,
	}, nil
}

// Dispatch creates pending deliveries and queues async jobs for subscribed
// tenant endpoints.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event) ([]Delivery, error) {
	if d == nil || d.endpoints == nil || d.store == nil {
		return nil, ErrStoreNotConfigured
	}
	ctx = normalizeContext(ctx)
	event, err := normalizeEvent(event, d.clock)
	if err != nil {
		return nil, err
	}
	if err := d.catalog.ValidateEvent(event); err != nil {
		return nil, err
	}
	endpoints, err := d.endpoints.ListEndpoints(ctx, event.TenantID, event.Type)
	if err != nil {
		return nil, err
	}
	now := d.clock().UTC()
	deliveries := make([]Delivery, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if !EndpointMatches(endpoint, event) {
			continue
		}
		if err := ValidateEndpoint(endpoint, d.endpointPolicy); err != nil {
			return nil, err
		}
		delivery := Delivery{
			ID:         strings.TrimSpace(d.deliveryIDFunc(event, endpoint)),
			TenantID:   event.TenantID,
			EndpointID: strings.TrimSpace(endpoint.ID),
			EventID:    event.ID,
			EventType:  event.Type,
			URL:        strings.TrimSpace(endpoint.URL),
			State:      StatePending,
			NextAt:     now,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := ValidateDelivery(delivery); err != nil {
			return nil, err
		}
		job := JobPayload{DeliveryID: delivery.ID, EndpointID: delivery.EndpointID, Event: event}
		if err := d.store.EnqueueDelivery(ctx, delivery, job); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, nil
}

// ValidateDelivery verifies required durable delivery fields.
func ValidateDelivery(delivery Delivery) error {
	if strings.TrimSpace(delivery.ID) == "" ||
		strings.TrimSpace(delivery.TenantID) == "" ||
		strings.TrimSpace(delivery.EndpointID) == "" ||
		strings.TrimSpace(delivery.EventID) == "" ||
		!validEventType(strings.TrimSpace(delivery.EventType), false) ||
		strings.TrimSpace(delivery.URL) == "" ||
		delivery.State == "" {
		return ErrInvalidDelivery
	}
	return nil
}

// EncodeJobPayload validates and encodes the safe async delivery payload.
func EncodeJobPayload(payload JobPayload) ([]byte, error) {
	if strings.TrimSpace(payload.DeliveryID) == "" || strings.TrimSpace(payload.EndpointID) == "" {
		return nil, ErrInvalidDelivery
	}
	event, err := normalizeEvent(payload.Event, func() time.Time { return time.Now().UTC() })
	if err != nil {
		return nil, err
	}
	payload.DeliveryID = strings.TrimSpace(payload.DeliveryID)
	payload.EndpointID = strings.TrimSpace(payload.EndpointID)
	payload.Event = event
	return json.Marshal(payload)
}

// DecodeJobPayload decodes and validates a worker job payload.
func DecodeJobPayload(payload []byte) (JobPayload, error) {
	var decoded JobPayload
	if len(payload) == 0 || !json.Valid(payload) {
		return JobPayload{}, ErrInvalidDelivery
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return JobPayload{}, err
	}
	if strings.TrimSpace(decoded.DeliveryID) == "" || strings.TrimSpace(decoded.EndpointID) == "" {
		return JobPayload{}, ErrInvalidDelivery
	}
	event, err := normalizeEvent(decoded.Event, func() time.Time { return time.Now().UTC() })
	if err != nil {
		return JobPayload{}, err
	}
	decoded.DeliveryID = strings.TrimSpace(decoded.DeliveryID)
	decoded.EndpointID = strings.TrimSpace(decoded.EndpointID)
	decoded.Event = event
	return decoded, nil
}

// HTTPDoer is the subset of http.Client used by Deliverer.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// DelivererConfig configures the HTTP webhook deliverer.
type DelivererConfig struct {
	Client          HTTPDoer
	Clock           func() time.Time
	EndpointPolicy  EndpointPolicy
	SignatureHeader string
	EventIDHeader   string
	TimestampHeader string
	UserAgent       string
	Metrics         MetricsRecorder
}

// Deliverer signs and sends outbound webhook HTTP requests.
type Deliverer struct {
	client          HTTPDoer
	clock           func() time.Time
	endpointPolicy  EndpointPolicy
	signatureHeader string
	eventIDHeader   string
	timestampHeader string
	userAgent       string
	metrics         MetricsRecorder
}

// NewDeliverer constructs an HTTP webhook deliverer.
func NewDeliverer(cfg DelivererConfig) (*Deliverer, error) {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	clock := cfg.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	userAgent := strings.TrimSpace(cfg.UserAgent)
	if userAgent == "" {
		userAgent = "api-toolkit-webhookdelivery/1"
	}
	return &Deliverer{
		client:          client,
		clock:           clock,
		endpointPolicy:  cfg.EndpointPolicy,
		signatureHeader: strings.TrimSpace(cfg.SignatureHeader),
		eventIDHeader:   strings.TrimSpace(cfg.EventIDHeader),
		timestampHeader: strings.TrimSpace(cfg.TimestampHeader),
		userAgent:       userAgent,
		metrics:         cfg.Metrics,
	}, nil
}

// Deliver signs and sends an event to an endpoint. The returned error and
// AttemptResult.Error are deliberately payload- and secret-free.
func (d *Deliverer) Deliver(ctx context.Context, endpoint Endpoint, event Event, attempt int) (AttemptResult, error) {
	ctx = normalizeContext(ctx)
	result := AttemptResult{
		TenantID:   strings.TrimSpace(event.TenantID),
		EndpointID: strings.TrimSpace(endpoint.ID),
		EventID:    strings.TrimSpace(event.ID),
		EventType:  strings.TrimSpace(event.Type),
		Attempt:    attempt,
		OccurredAt: d.now(),
	}
	if d == nil || d.client == nil {
		result.Error = "webhook deliverer not configured"
		d.observe(ctx, result, "validation_error")
		return result, fmt.Errorf("%w: %s", ErrDeliveryFailed, result.Error)
	}
	if result.Attempt <= 0 {
		result.Attempt = 1
	}
	event, err := normalizeEvent(event, d.clock)
	if err != nil {
		result.Error = "webhook event invalid"
		d.observe(ctx, result, "validation_error")
		return result, err
	}
	if err := ValidateEndpoint(endpoint, d.endpointPolicy); err != nil {
		result.Error = "webhook endpoint invalid"
		d.observe(ctx, result, "validation_error")
		return result, err
	}
	if !EndpointMatches(endpoint, event) {
		result.Error = "webhook endpoint is not subscribed to event"
		d.observe(ctx, result, "validation_error")
		return result, ErrTenantMismatch
	}
	headers, err := SafeHeaders(endpoint.Headers)
	if err != nil {
		result.Error = "webhook headers invalid"
		d.observe(ctx, result, "validation_error")
		return result, err
	}
	headers.Set("User-Agent", d.userAgent)
	signer, err := webhooks.NewHMACSHA256Signer(webhooks.HMACSignerConfig{Secret: endpoint.SigningSecret, Prefix: "sha256="})
	if err != nil {
		result.Error = "webhook signer invalid"
		d.observe(ctx, result, "validation_error")
		return result, fmt.Errorf("%w: %s", ErrDeliveryFailed, result.Error)
	}
	req, err := webhooks.BuildSignedRequest(ctx, webhooks.OutgoingEvent[json.RawMessage]{
		ID:      event.ID,
		Type:    event.Type,
		Payload: event.Payload,
	}, webhooks.SignedRequestConfig{
		URL:             strings.TrimSpace(endpoint.URL),
		Signer:          signer,
		EventID:         event.ID,
		Timestamp:       d.now(),
		SignatureHeader: d.signatureHeader,
		EventIDHeader:   d.eventIDHeader,
		TimestampHeader: d.timestampHeader,
		Headers:         headers,
	})
	if err != nil {
		result.Error = "webhook request invalid"
		d.observe(ctx, result, "validation_error")
		return result, fmt.Errorf("%w: %s", ErrDeliveryFailed, result.Error)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		result.StatusClass = "transport"
		result.Retryable = true
		result.Error = "webhook delivery transport failed"
		d.observe(ctx, result, "transport_error")
		return result, fmt.Errorf("%w: %s", ErrDeliveryFailed, result.Error)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	result.StatusCode = resp.StatusCode
	result.StatusClass = statusClass(resp.StatusCode)
	result.Accepted = resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	result.Retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
	if result.Accepted {
		d.observe(ctx, result, "accepted")
		return result, nil
	}
	result.Error = "webhook delivery failed with status class " + result.StatusClass
	d.observe(ctx, result, "failed")
	return result, fmt.Errorf("%w: %s", ErrDeliveryFailed, result.Error)
}

func (d *Deliverer) now() time.Time {
	if d == nil || d.clock == nil {
		return time.Now().UTC()
	}
	return d.clock().UTC()
}

func (d *Deliverer) observe(ctx context.Context, result AttemptResult, outcome string) {
	if d == nil || d.metrics == nil {
		return
	}
	d.metrics.ObserveWebhookDelivery(ctx, DeliveryObservation{
		EventType:   SafeLabel(result.EventType),
		Outcome:     SafeLabel(outcome),
		StatusClass: SafeLabel(result.StatusClass),
	})
}

// HandlerConfig configures an async delivery handler.
type HandlerConfig struct {
	Endpoints EndpointGetter
	Deliverer *Deliverer
	Attempts  AttemptRecorder
}

// Handler loads endpoints, sends jobs, and records attempt metadata for async.Runner.
type Handler struct {
	endpoints EndpointGetter
	deliverer *Deliverer
	attempts  AttemptRecorder
}

// NewHandler constructs an async delivery handler.
func NewHandler(cfg HandlerConfig) (*Handler, error) {
	if cfg.Endpoints == nil || cfg.Deliverer == nil {
		return nil, ErrStoreNotConfigured
	}
	return &Handler{endpoints: cfg.Endpoints, deliverer: cfg.Deliverer, attempts: cfg.Attempts}, nil
}

// Handle executes one async delivery job.
func (h *Handler) Handle(ctx context.Context, job async.Job) error {
	if h == nil || h.endpoints == nil || h.deliverer == nil {
		return ErrStoreNotConfigured
	}
	ctx = normalizeContext(ctx)
	payload, err := DecodeJobPayload(job.Payload)
	if err != nil {
		return err
	}
	if strings.TrimSpace(job.ID) != "" && strings.TrimSpace(job.ID) != payload.DeliveryID {
		return ErrInvalidDelivery
	}
	if strings.TrimSpace(job.TenantID) == "" || strings.TrimSpace(job.TenantID) != payload.Event.TenantID {
		return ErrTenantMismatch
	}
	endpoint, ok, err := h.endpoints.GetEndpoint(ctx, payload.Event.TenantID, payload.EndpointID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeliveryNotFound
	}
	if !EndpointMatches(endpoint, payload.Event) {
		return ErrTenantMismatch
	}
	result, deliveryErr := h.deliverer.Deliver(ctx, endpoint, payload.Event, job.Attempts+1)
	result.DeliveryID = payload.DeliveryID
	if h.attempts != nil {
		if recordErr := h.attempts.RecordAttempt(ctx, result); recordErr != nil {
			return errors.Join(deliveryErr, recordErr)
		}
	}
	return deliveryErr
}

var _ async.Handler = (*Handler)(nil)

// RetryPolicy computes bounded exponential retry delays.
type RetryPolicy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// NextDelay returns the retry delay for a one-based attempt number.
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	base := p.BaseDelay
	if base <= 0 {
		base = defaultBaseDelay
	}
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = defaultMaxDelay
	}
	if attempt <= 1 {
		if base > maxDelay {
			return maxDelay
		}
		return base
	}
	delay := base
	for i := 1; i < attempt; i++ {
		if delay >= maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// ReplayCommand requests tenant-scoped redelivery of a durable delivery.
type ReplayCommand struct {
	TenantID   string
	DeliveryID string
	NextAt     time.Time
}

// Replay schedules a tenant-scoped delivery for another attempt.
func Replay(ctx context.Context, store Replayer, cmd ReplayCommand) error {
	if store == nil {
		return ErrStoreNotConfigured
	}
	tenantID := strings.TrimSpace(cmd.TenantID)
	deliveryID := strings.TrimSpace(cmd.DeliveryID)
	if tenantID == "" || deliveryID == "" {
		return ErrInvalidDelivery
	}
	return store.ReplayDelivery(normalizeContext(ctx), tenantID, deliveryID, cmd.NextAt)
}

// SafeLabel returns a bounded metric/log label.
func SafeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
		if out.Len() >= maxLabelLength {
			break
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}

func normalizeEvent(event Event, clock func() time.Time) (Event, error) {
	event.ID = strings.TrimSpace(event.ID)
	event.TenantID = strings.TrimSpace(event.TenantID)
	event.Type = strings.TrimSpace(event.Type)
	if event.OccurredAt.IsZero() {
		if clock == nil {
			clock = func() time.Time { return time.Now().UTC() }
		}
		event.OccurredAt = clock().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	if err := ValidateEvent(event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func validateEndpointURL(rawURL string, policy EndpointPolicy) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || containsControl(rawURL) {
		return ErrInvalidEndpoint
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return ErrInvalidEndpoint
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		if policy.AllowInsecureHTTP {
			return nil
		}
		return ErrInvalidEndpoint
	default:
		return ErrInvalidEndpoint
	}
}

func validEventType(value string, allowWildcard bool) bool {
	if value == AnyEventType {
		return allowWildcard
	}
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func unsafeHeaderName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization",
		"cookie",
		"set-cookie",
		"proxy-authorization",
		"connection",
		"transfer-encoding",
		"upgrade",
		"host",
		"x-signature",
		"x-hook-signature",
		"x-webhook-signature",
		"x-webhook-event-id",
		"x-webhook-timestamp":
		return true
	default:
		return false
	}
}

func containsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func statusClass(status int) string {
	if status < 100 || status > 999 {
		return "unknown"
	}
	return fmt.Sprintf("%dxx", status/100)
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
