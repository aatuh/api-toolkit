//go:build postgres

package webhookdeliverypostgres

import (
	"context"
	"strings"
	"testing"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
)

func TestStorePersistsWebhookDeliveriesAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "webhook-delivery",
		SQL: `CREATE TABLE webhook_endpoints (
			id text PRIMARY KEY, organization_id text NOT NULL, url text NOT NULL,
			event_types text[] NOT NULL, disabled_at timestamptz, created_at timestamptz NOT NULL
		);
		CREATE TABLE webhook_deliveries (
			id text PRIMARY KEY, organization_id text NOT NULL, endpoint_id text NOT NULL,
			event_id text NOT NULL, event_type text NOT NULL, payload bytea NOT NULL,
			state text NOT NULL, attempts integer NOT NULL, next_at timestamptz NOT NULL,
			last_status_code integer NOT NULL, last_error text, created_at timestamptz NOT NULL,
			delivered_at timestamptz
		);
		CREATE TABLE outbox_events (
			id text PRIMARY KEY, organization_id text NOT NULL, event_type text NOT NULL,
			payload bytea NOT NULL, state text NOT NULL, next_at timestamptz NOT NULL,
			created_at timestamptz NOT NULL
		)`,
	}); err != nil {
		t.Fatalf("create webhook schema: %v", err)
	}
	endpointID := h.NextText("endpoint")
	if _, err := h.Pool.Exec(ctx, "INSERT INTO webhook_endpoints (id, organization_id, url, event_types, created_at) VALUES ($1, $2, $3, $4, $5)", endpointID, "org-test", "https://example.test/hooks", []string{"widget.created"}, h.FixedTime()); err != nil {
		t.Fatalf("insert endpoint: %v", err)
	}
	store := New(&pgxpooladapter.Adapter{Pool: h.Pool}, Options{
		Clock: h.FixedTime,
		SecretResolver: SecretResolverFunc(func(_ context.Context, tenantID, gotEndpointID string) ([]byte, bool, error) {
			return []byte("test-signing-secret"), tenantID == "org-test" && gotEndpointID == endpointID, nil
		}),
	})
	endpoints, err := store.ListEndpoints(ctx, "org-test", "widget.created")
	if err != nil || len(endpoints) != 1 || string(endpoints[0].SigningSecret) != "test-signing-secret" {
		t.Fatalf("ListEndpoints() = (%#v, %v)", endpoints, err)
	}
	deliveryID := h.NextText("delivery")
	event := webhookdelivery.Event{ID: h.NextText("event"), TenantID: "org-test", Type: "widget.created", Payload: []byte(`{"id":"widget-test"}`)}
	delivery := webhookdelivery.Delivery{ID: deliveryID, TenantID: "org-test", EndpointID: endpointID, EventID: event.ID, EventType: event.Type, URL: endpoints[0].URL}
	job := webhookdelivery.JobPayload{DeliveryID: deliveryID, EndpointID: endpointID, Event: event}
	if err := store.EnqueueDelivery(ctx, delivery, job); err != nil {
		t.Fatalf("EnqueueDelivery() error = %v", err)
	}
	if err := store.EnqueueDelivery(ctx, delivery, job); err == nil {
		t.Fatal("duplicate delivery was accepted")
	}
	var outboxCount int
	if err := h.Pool.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE id = $1", deliveryID).Scan(&outboxCount); err != nil || outboxCount != 1 {
		t.Fatalf("outbox count = (%d, %v), want 1", outboxCount, err)
	}
	if err := store.RecordAttempt(ctx, webhookdelivery.AttemptResult{DeliveryID: deliveryID, TenantID: "org-test", EndpointID: endpointID, EventID: event.ID, EventType: event.Type, Attempt: 1, StatusCode: 503, Retryable: true, Error: "authorization: must not persist"}); err != nil {
		t.Fatalf("RecordAttempt() error = %v", err)
	}
	var lastError string
	if err := h.Pool.QueryRow(ctx, "SELECT last_error FROM webhook_deliveries WHERE id = $1", deliveryID).Scan(&lastError); err != nil {
		t.Fatalf("read attempt result: %v", err)
	}
	if strings.Contains(strings.ToLower(lastError), "authorization") || lastError != "webhook delivery failed" {
		t.Fatalf("stored attempt error = %q", lastError)
	}
	if err := store.ReplayDelivery(ctx, "org-test", deliveryID, h.FixedTime()); err != nil {
		t.Fatalf("ReplayDelivery() error = %v", err)
	}
}
