package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
)

var (
	ErrWebhookStoreRequired    = errors.New("postgres webhook store db is required")
	ErrWebhookInvalid          = errors.New("postgres webhook record is invalid")
	ErrWebhookSecretKeyInvalid = errors.New("WEBHOOK_SECRET_KEY must decode to 32 bytes")
)

type WebhookStore struct {
	pool           contracts.DatabasePool
	base           *webhookdeliverypostgres.Store
	secretKey      []byte
	now            func() time.Time
	tx             contracts.TxManager
	endpointPolicy webhookdelivery.EndpointPolicy
}

func NewWebhookStore(pool contracts.DatabasePool, secretKey string, endpointPolicy webhookdelivery.EndpointPolicy) (*WebhookStore, error) {
	key, err := decodeWebhookSecretKey(secretKey)
	if err != nil {
		return nil, err
	}
	store := &WebhookStore{
		pool:           pool,
		secretKey:      key,
		now:            time.Now,
		tx:             txpostgres.New(pool),
		endpointPolicy: endpointPolicy,
	}
	store.base = webhookdeliverypostgres.New(pool, webhookdeliverypostgres.Options{
		SecretResolver: webhookdeliverypostgres.SecretResolverFunc(store.resolveSigningSecret),
		EndpointPolicy: endpointPolicy,
	})
	return store, nil
}

func (s *WebhookStore) CreateEndpoint(ctx context.Context, endpoint webhookdelivery.Endpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pool == nil {
		return ErrWebhookStoreRequired
	}
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = s.now().UTC()
	}
	if endpoint.UpdatedAt.IsZero() {
		endpoint.UpdatedAt = endpoint.CreatedAt
	}
	if err := webhookdelivery.ValidateEndpoint(endpoint, s.endpointPolicy); err != nil {
		return ErrWebhookInvalid
	}
	ciphertext, nonce, err := s.encryptWebhookSecret(endpoint.SigningSecret)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(endpoint.SigningSecret)
	db := txpostgres.FromCtx(ctx, s.pool)
	_, err = db.Exec(ctx,
		"insert into webhook_endpoints (id, organization_id, url, event_types, secret_hash, secret_ciphertext, secret_nonce, created_at) values ($1, $2, $3, $4, $5, $6, $7, $8)",
		strings.TrimSpace(endpoint.ID),
		strings.TrimSpace(endpoint.TenantID),
		strings.TrimSpace(endpoint.URL),
		append([]string(nil), endpoint.Events...),
		hash[:],
		ciphertext,
		nonce,
		endpoint.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}
	return nil
}

func (s *WebhookStore) ListEndpointsForActor(ctx context.Context, tenantID string) ([]webhookdelivery.Endpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	rows, err := db.Query(ctx, "select id, organization_id, url, event_types, disabled_at is not null, created_at from webhook_endpoints where organization_id=$1 order by id", tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()
	var out []webhookdelivery.Endpoint
	for rows.Next() {
		endpoint, err := scanPublicWebhookEndpoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list webhook endpoint rows: %w", err)
	}
	return out, nil
}

func (s *WebhookStore) ListEndpoints(ctx context.Context, tenantID, eventType string) ([]webhookdelivery.Endpoint, error) {
	if s == nil || s.base == nil {
		return nil, ErrWebhookStoreRequired
	}
	return s.base.ListEndpoints(ctx, tenantID, eventType)
}

func (s *WebhookStore) GetEndpoint(ctx context.Context, tenantID, endpointID string) (webhookdelivery.Endpoint, bool, error) {
	if s == nil || s.base == nil {
		return webhookdelivery.Endpoint{}, false, ErrWebhookStoreRequired
	}
	return s.base.GetEndpoint(ctx, tenantID, endpointID)
}

func (s *WebhookStore) EnqueueDelivery(ctx context.Context, delivery webhookdelivery.Delivery, job webhookdelivery.JobPayload) error {
	if s == nil || s.base == nil {
		return ErrWebhookStoreRequired
	}
	return s.base.EnqueueDelivery(ctx, delivery, job)
}

func (s *WebhookStore) RecordAttempt(ctx context.Context, result webhookdelivery.AttemptResult) error {
	if s == nil || s.base == nil {
		return ErrWebhookStoreRequired
	}
	return s.base.RecordAttempt(ctx, result)
}

func (s *WebhookStore) ListDeliveries(ctx context.Context, tenantID string) ([]webhookdelivery.Delivery, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.pool == nil {
		return nil, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	rows, err := db.Query(ctx, "select d.id, d.organization_id, d.endpoint_id, d.event_id, d.event_type, e.url, d.state, d.attempts, d.next_at, coalesce(d.last_status_code, 0), coalesce(d.last_error, ''), d.created_at, coalesce(d.delivered_at, d.created_at) from webhook_deliveries d join webhook_endpoints e on e.id=d.endpoint_id and e.organization_id=d.organization_id where d.organization_id=$1 order by d.id", tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	defer rows.Close()
	var out []webhookdelivery.Delivery
	for rows.Next() {
		delivery, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list webhook delivery rows: %w", err)
	}
	return out, nil
}

func (s *WebhookStore) GetDelivery(ctx context.Context, tenantID, deliveryID string) (webhookdelivery.Delivery, bool, error) {
	if err := ctx.Err(); err != nil {
		return webhookdelivery.Delivery{}, false, err
	}
	if s == nil || s.pool == nil {
		return webhookdelivery.Delivery{}, false, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return webhookdelivery.Delivery{}, false, ErrWebhookInvalid
	}
	db := txpostgres.FromCtx(ctx, s.pool)
	row := db.QueryRow(ctx, "select d.id, d.organization_id, d.endpoint_id, d.event_id, d.event_type, e.url, d.state, d.attempts, d.next_at, coalesce(d.last_status_code, 0), coalesce(d.last_error, ''), d.created_at, coalesce(d.delivered_at, d.created_at) from webhook_deliveries d join webhook_endpoints e on e.id=d.endpoint_id and e.organization_id=d.organization_id where d.organization_id=$1 and d.id=$2", tenantID, deliveryID)
	delivery, err := scanWebhookDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhookdelivery.Delivery{}, false, nil
	}
	if err != nil {
		return webhookdelivery.Delivery{}, false, fmt.Errorf("get webhook delivery: %w", err)
	}
	return delivery, true, nil
}

func (s *WebhookStore) ReplayDelivery(ctx context.Context, tenantID, deliveryID string, nextAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.pool == nil || s.tx == nil {
		return ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	deliveryID = strings.TrimSpace(deliveryID)
	if tenantID == "" || deliveryID == "" {
		return webhookdelivery.ErrInvalidDelivery
	}
	if nextAt.IsZero() {
		nextAt = s.now().UTC()
	}

	var (
		endpointID string
		eventID    string
		eventType  string
		payload    []byte
	)
	db := txpostgres.FromCtx(ctx, s.pool)
	err := db.QueryRow(ctx, "select endpoint_id, event_id, event_type, payload from webhook_deliveries where organization_id=$1 and id=$2", tenantID, deliveryID).Scan(&endpointID, &eventID, &eventType, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhookdelivery.ErrDeliveryNotFound
	}
	if err != nil {
		return fmt.Errorf("load webhook delivery replay payload: %w", err)
	}
	jobPayload, err := webhookdelivery.EncodeJobPayload(webhookdelivery.JobPayload{
		DeliveryID: deliveryID,
		EndpointID: strings.TrimSpace(endpointID),
		Event: webhookdelivery.Event{
			ID:       strings.TrimSpace(eventID),
			TenantID: tenantID,
			Type:     strings.TrimSpace(eventType),
			Payload:  append([]byte(nil), payload...),
		},
	})
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		db := txpostgres.FromCtx(ctx, s.pool)
		res, err := db.Exec(ctx, "update webhook_deliveries set state='pending', next_at=$3, last_error=null where organization_id=$1 and id=$2", tenantID, deliveryID, nextAt.UTC())
		if err != nil {
			return fmt.Errorf("replay webhook delivery: %w", err)
		}
		if res == nil || res.RowsAffected() == 0 {
			return webhookdelivery.ErrDeliveryNotFound
		}
		_, err = db.Exec(ctx,
			"insert into outbox_events (id, organization_id, event_type, payload, state, next_at, created_at) values ($1, $2, $3, $4, 'pending', $5, $6) on conflict (id) do update set event_type=excluded.event_type, payload=excluded.payload, state='pending', lease_owner=null, lease_expires_at=null, retry_count=0, next_at=excluded.next_at",
			deliveryID,
			tenantID,
			webhookdeliverypostgres.OutboxEventType,
			jobPayload,
			nextAt.UTC(),
			s.now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("requeue webhook delivery: %w", err)
		}
		return nil
	})
}

func (s *WebhookStore) resolveSigningSecret(ctx context.Context, tenantID, endpointID string) ([]byte, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, ErrWebhookStoreRequired
	}
	tenantID = strings.TrimSpace(tenantID)
	endpointID = strings.TrimSpace(endpointID)
	if tenantID == "" || endpointID == "" {
		return nil, false, ErrWebhookInvalid
	}
	var ciphertext, nonce []byte
	db := txpostgres.FromCtx(ctx, s.pool)
	err := db.QueryRow(ctx, "select secret_ciphertext, secret_nonce from webhook_endpoints where organization_id=$1 and id=$2 and disabled_at is null", tenantID, endpointID).Scan(&ciphertext, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load webhook signing secret: %w", err)
	}
	secret, err := s.decryptWebhookSecret(ciphertext, nonce)
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (s *WebhookStore) encryptWebhookSecret(secret []byte) ([]byte, []byte, error) {
	if len(secret) == 0 || len(s.secretKey) != 32 {
		return nil, nil, ErrWebhookSecretKeyInvalid
	}
	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, secret, nil), nonce, nil
}

func (s *WebhookStore) decryptWebhookSecret(ciphertext, nonce []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 || len(s.secretKey) != 32 {
		return nil, ErrWebhookSecretKeyInvalid
	}
	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrWebhookSecretKeyInvalid
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func decodeWebhookSecretKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrWebhookSecretKeyInvalid
	}
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == 32 {
			return append([]byte(nil), decoded...), nil
		}
	}
	if len(value) == 32 {
		return []byte(value), nil
	}
	return nil, ErrWebhookSecretKeyInvalid
}

type webhookScanner interface {
	Scan(dest ...any) error
}

func scanPublicWebhookEndpoint(row webhookScanner) (webhookdelivery.Endpoint, error) {
	var endpoint webhookdelivery.Endpoint
	if err := row.Scan(&endpoint.ID, &endpoint.TenantID, &endpoint.URL, &endpoint.Events, &endpoint.Disabled, &endpoint.CreatedAt); err != nil {
		return webhookdelivery.Endpoint{}, err
	}
	endpoint.ID = strings.TrimSpace(endpoint.ID)
	endpoint.TenantID = strings.TrimSpace(endpoint.TenantID)
	endpoint.URL = strings.TrimSpace(endpoint.URL)
	endpoint.Events = append([]string(nil), endpoint.Events...)
	endpoint.CreatedAt = endpoint.CreatedAt.UTC()
	if endpoint.ID == "" || endpoint.TenantID == "" || endpoint.URL == "" || len(endpoint.Events) == 0 {
		return webhookdelivery.Endpoint{}, ErrWebhookInvalid
	}
	return endpoint, nil
}

func scanWebhookDelivery(row webhookScanner) (webhookdelivery.Delivery, error) {
	var (
		delivery webhookdelivery.Delivery
		state    string
	)
	if err := row.Scan(
		&delivery.ID,
		&delivery.TenantID,
		&delivery.EndpointID,
		&delivery.EventID,
		&delivery.EventType,
		&delivery.URL,
		&state,
		&delivery.Attempt,
		&delivery.NextAt,
		&delivery.LastStatusCode,
		&delivery.LastError,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	); err != nil {
		return webhookdelivery.Delivery{}, err
	}
	delivery.State = webhookdelivery.DeliveryState(strings.TrimSpace(state))
	delivery.NextAt = delivery.NextAt.UTC()
	delivery.CreatedAt = delivery.CreatedAt.UTC()
	delivery.UpdatedAt = delivery.UpdatedAt.UTC()
	if err := webhookdelivery.ValidateDelivery(delivery); err != nil {
		return webhookdelivery.Delivery{}, ErrWebhookInvalid
	}
	return delivery, nil
}
