package webhookdeliverypostgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v2/webhookdelivery"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/ports"
)

const (
	defaultEndpointTable = "webhook_endpoints"
	defaultDeliveryTable = "webhook_deliveries"
	defaultOutboxTable   = "outbox_events"

	// OutboxEventType is the durable async job type used for webhook deliveries.
	OutboxEventType = "webhook.delivery"
)

var (
	// ErrInvalidTable reports that a configured table name is not a safe SQL identifier.
	ErrInvalidTable = errors.New("invalid webhook delivery table")
	// ErrStoreNotConfigured reports that the store or pool was not configured.
	ErrStoreNotConfigured = errors.New("webhook delivery postgres store not configured")
	// ErrSecretResolverRequired reports that endpoint signing secrets cannot be loaded.
	ErrSecretResolverRequired = errors.New("webhook signing secret resolver required")
	// ErrSecretNotFound reports that a tenant-scoped endpoint secret is missing.
	ErrSecretNotFound = errors.New("webhook signing secret not found")
	// ErrDeliveryNotFound reports a missing tenant-scoped delivery row.
	ErrDeliveryNotFound = errors.New("webhook delivery not found")
)

// SecretResolver loads endpoint signing material from application-owned secret storage.
type SecretResolver interface {
	ResolveSigningSecret(ctx context.Context, tenantID, endpointID string) ([]byte, bool, error)
}

// SecretResolverFunc adapts a function to SecretResolver.
type SecretResolverFunc func(context.Context, string, string) ([]byte, bool, error)

// ResolveSigningSecret loads endpoint signing material.
func (f SecretResolverFunc) ResolveSigningSecret(ctx context.Context, tenantID, endpointID string) ([]byte, bool, error) {
	if f == nil {
		return nil, false, ErrSecretResolverRequired
	}
	return f(ctx, tenantID, endpointID)
}

// Options configures the Postgres webhook delivery store.
type Options struct {
	EndpointTable  string
	DeliveryTable  string
	OutboxTable    string
	Clock          func() time.Time
	SecretResolver SecretResolver
	TxManager      ports.TxManager
}

// Store implements tenant-scoped webhook endpoint lookup, delivery enqueue,
// attempt recording, and replay scheduling over Postgres tables.
type Store struct {
	Pool           ports.DatabasePool
	endpointTable  string
	deliveryTable  string
	outboxTable    string
	clock          func() time.Time
	secretResolver SecretResolver
	tx             ports.TxManager
}

// New creates a Postgres-backed webhook delivery store.
func New(pool ports.DatabasePool, opts Options) *Store {
	endpointTable := strings.TrimSpace(opts.EndpointTable)
	if endpointTable == "" {
		endpointTable = defaultEndpointTable
	}
	deliveryTable := strings.TrimSpace(opts.DeliveryTable)
	if deliveryTable == "" {
		deliveryTable = defaultDeliveryTable
	}
	outboxTable := strings.TrimSpace(opts.OutboxTable)
	if outboxTable == "" {
		outboxTable = defaultOutboxTable
	}
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	tx := opts.TxManager
	if tx == nil {
		tx = txpostgres.New(pool)
	}
	return &Store{
		Pool:           pool,
		endpointTable:  endpointTable,
		deliveryTable:  deliveryTable,
		outboxTable:    outboxTable,
		clock:          clock,
		secretResolver: opts.SecretResolver,
		tx:             tx,
	}
}

// ListEndpoints returns enabled tenant endpoints subscribed to eventType.
func (s *Store) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrStoreNotConfigured
	}
	tenantID = strings.TrimSpace(tenantID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || !validEventType(eventType) {
		return nil, webhookdelivery.ErrInvalidEvent
	}
	table, err := quoteTable(s.endpointTable)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`select id, organization_id, url, event_types, disabled_at is not null, created_at from %s where organization_id=$1 and disabled_at is null and ($2 = any(event_types) or $3 = any(event_types)) order by id`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	rows, err := db.Query(ctx, query, tenantID, eventType, webhookdelivery.AnyEventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []webhookdelivery.Endpoint
	for rows.Next() {
		endpoint, err := s.scanEndpointRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return endpoints, nil
}

// GetEndpoint returns one tenant-scoped endpoint.
func (s *Store) GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if s == nil || s.Pool == nil {
		return webhookdelivery.Endpoint{}, false, ErrStoreNotConfigured
	}
	tenantID = strings.TrimSpace(tenantID)
	endpointID = strings.TrimSpace(endpointID)
	if tenantID == "" || endpointID == "" {
		return webhookdelivery.Endpoint{}, false, webhookdelivery.ErrInvalidEndpoint
	}
	table, err := quoteTable(s.endpointTable)
	if err != nil {
		return webhookdelivery.Endpoint{}, false, err
	}
	query := fmt.Sprintf(`select id, organization_id, url, event_types, disabled_at is not null, created_at from %s where organization_id=$1 and id=$2`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	row := db.QueryRow(ctx, query, tenantID, endpointID)
	endpoint, err := s.scanEndpoint(ctx, row)
	if txpostgres.IsNoRows(err) {
		return webhookdelivery.Endpoint{}, false, nil
	}
	if err != nil {
		return webhookdelivery.Endpoint{}, false, err
	}
	return endpoint, true, nil
}

// EnqueueDelivery inserts a delivery row and matching outbox job in one transaction.
func (s *Store) EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	if s == nil || s.Pool == nil || s.tx == nil {
		return ErrStoreNotConfigured
	}
	if delivery.State == "" {
		delivery.State = webhookdelivery.StatePending
	}
	if delivery.NextAt.IsZero() {
		delivery.NextAt = s.clock()
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = s.clock()
	}
	if delivery.UpdatedAt.IsZero() {
		delivery.UpdatedAt = delivery.CreatedAt
	}
	if err := webhookdelivery.ValidateDelivery(delivery); err != nil {
		return err
	}
	if strings.TrimSpace(job.DeliveryID) != strings.TrimSpace(delivery.ID) ||
		strings.TrimSpace(job.EndpointID) != strings.TrimSpace(delivery.EndpointID) ||
		strings.TrimSpace(job.Event.TenantID) != strings.TrimSpace(delivery.TenantID) ||
		strings.TrimSpace(job.Event.ID) != strings.TrimSpace(delivery.EventID) ||
		strings.TrimSpace(job.Event.Type) != strings.TrimSpace(delivery.EventType) {
		return webhookdelivery.ErrTenantMismatch
	}
	jobPayload, err := webhookdelivery.EncodeJobPayload(job)
	if err != nil {
		return err
	}
	deliveryTable, err := quoteTable(s.deliveryTable)
	if err != nil {
		return err
	}
	outboxTable, err := quoteTable(s.outboxTable)
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		db := txpostgres.FromCtx(ctx, s.Pool)
		insertDelivery := fmt.Sprintf(`insert into %s (id, organization_id, endpoint_id, event_id, event_type, payload, state, attempts, next_at, last_status_code, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, deliveryTable) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
		if _, err := db.Exec(ctx, insertDelivery, delivery.ID, delivery.TenantID, delivery.EndpointID, delivery.EventID, delivery.EventType, []byte(job.Event.Payload), string(delivery.State), delivery.Attempt, delivery.NextAt, delivery.LastStatusCode, delivery.CreatedAt); err != nil {
			return err
		}
		insertOutbox := fmt.Sprintf(`insert into %s (id, organization_id, event_type, payload, state, next_at, created_at) values ($1,$2,$3,$4,'pending',$5,$6)`, outboxTable) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
		_, err := db.Exec(ctx, insertOutbox, delivery.ID, delivery.TenantID, OutboxEventType, jobPayload, delivery.NextAt, delivery.CreatedAt)
		return err
	})
}

// RecordAttempt stores a sanitized delivery attempt outcome.
func (s *Store) RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	if err := validateAttemptResult(result); err != nil {
		return err
	}
	table, err := quoteTable(s.deliveryTable)
	if err != nil {
		return err
	}
	state := webhookdelivery.StateFailed
	if result.Accepted {
		state = webhookdelivery.StateSucceeded
	} else if !result.Retryable {
		state = webhookdelivery.StateDeadLetter
	}
	now := s.clock()
	lastError := safeAttemptError(result.Error)
	query := fmt.Sprintf(`update %s set attempts=$4, state=$5, last_status_code=$6, last_error=$7, delivered_at=case when $5='succeeded' then $8 else delivered_at end where id=$1 and organization_id=$2 and endpoint_id=$3`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	res, err := db.Exec(ctx, query, result.DeliveryID, result.TenantID, result.EndpointID, result.Attempt, string(state), result.StatusCode, lastError, now)
	if err != nil {
		return err
	}
	if res == nil || res.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// ReplayDelivery schedules a tenant-scoped delivery for another attempt.
func (s *Store) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if s == nil || s.Pool == nil {
		return ErrStoreNotConfigured
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return webhookdelivery.ErrInvalidDelivery
	}
	if nextAt.IsZero() {
		nextAt = s.clock()
	}
	table, err := quoteTable(s.deliveryTable)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`update %s set state='pending', next_at=$3, last_error=null where organization_id=$1 and id=$2`, table) // #nosec G201 -- table is validated and quoted by quoteTable before interpolation.
	db := txpostgres.FromCtx(ctx, s.Pool)
	res, err := db.Exec(ctx, query, tenantID, deliveryID, nextAt)
	if err != nil {
		return err
	}
	if res == nil || res.RowsAffected() == 0 {
		return ErrDeliveryNotFound
	}
	return nil
}

// HealthChecker returns a Postgres webhook delivery dependency health checker.
func (s *Store) HealthChecker() ports.HealthChecker {
	var pool ports.DatabasePool
	if s != nil {
		pool = s.Pool
	}
	return health.NewCustomChecker(
		"postgres-webhook-delivery",
		func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
			if pool == nil {
				return ports.HealthStatusUnhealthy, "postgres webhook delivery pool not configured", nil
			}
			if err := pool.Ping(ctx); err != nil {
				return ports.HealthStatusUnhealthy, fmt.Sprintf("postgres webhook delivery ping failed: %v", err), nil
			}
			return ports.HealthStatusHealthy, "postgres webhook delivery healthy", nil
		},
	)
}

func (s *Store) scanEndpointRow(ctx context.Context, rows ports.DatabaseRows) (webhookdelivery.Endpoint, error) {
	var endpoint webhookdelivery.Endpoint
	if err := rows.Scan(&endpoint.ID, &endpoint.TenantID, &endpoint.URL, &endpoint.Events, &endpoint.Disabled, &endpoint.CreatedAt); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	secret, err := s.resolveSecret(ctx, endpoint.TenantID, endpoint.ID)
	if err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	endpoint.SigningSecret = secret
	if err := webhookdelivery.ValidateEndpoint(endpoint, webhookdelivery.EndpointPolicy{}); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	return endpoint, nil
}

func (s *Store) scanEndpoint(ctx context.Context, row ports.DatabaseRow) (webhookdelivery.Endpoint, error) {
	var endpoint webhookdelivery.Endpoint
	if err := row.Scan(&endpoint.ID, &endpoint.TenantID, &endpoint.URL, &endpoint.Events, &endpoint.Disabled, &endpoint.CreatedAt); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	secret, err := s.resolveSecret(ctx, endpoint.TenantID, endpoint.ID)
	if err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	endpoint.SigningSecret = secret
	if err := webhookdelivery.ValidateEndpoint(endpoint, webhookdelivery.EndpointPolicy{}); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	return endpoint, nil
}

func (s *Store) resolveSecret(ctx context.Context, tenantID, endpointID string) ([]byte, error) {
	if s.secretResolver == nil {
		return nil, ErrSecretResolverRequired
	}
	secret, ok, err := s.secretResolver.ResolveSigningSecret(ctx, tenantID, endpointID)
	if err != nil {
		return nil, err
	}
	if !ok || len(secret) == 0 {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), secret...), nil
}

func validateAttemptResult(result webhookdelivery.AttemptResult) error {
	if strings.TrimSpace(result.DeliveryID) == "" ||
		strings.TrimSpace(result.TenantID) == "" ||
		strings.TrimSpace(result.EndpointID) == "" ||
		strings.TrimSpace(result.EventID) == "" ||
		!validEventType(result.EventType) ||
		result.Attempt <= 0 {
		return webhookdelivery.ErrInvalidDelivery
	}
	return nil
}

func safeAttemptError(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if unsafeToken(value) {
		return "webhook delivery failed"
	}
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

func unsafeToken(value string) bool {
	normalized := strings.ToLower(value)
	for _, token := range []string{"authorization", "bearer ", "cookie", "password", "private_key", "secret", "token", "api_key", "apikey", "pepper"} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func quoteTable(name string) (string, error) {
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return "", ErrInvalidTable
	}
	for i, part := range parts {
		if !validIdentifier(part) {
			return "", ErrInvalidTable
		}
		parts[i] = `"` + part + `"`
	}
	return strings.Join(parts, "."), nil
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validEventType(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == webhookdelivery.AnyEventType || len(value) > 128 {
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

var _ webhookdelivery.EndpointRegistry = (*Store)(nil)
var _ webhookdelivery.EndpointGetter = (*Store)(nil)
var _ webhookdelivery.DeliveryEnqueuer = (*Store)(nil)
var _ webhookdelivery.AttemptRecorder = (*Store)(nil)
var _ webhookdelivery.Replayer = (*Store)(nil)
