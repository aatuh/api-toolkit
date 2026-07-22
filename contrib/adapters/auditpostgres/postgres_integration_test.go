//go:build postgres

package auditpostgres

import (
	"context"
	"errors"
	"testing"
	"time"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/audit"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestStorePersistsAuditEventsAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "audit-events",
		SQL: `CREATE TABLE audit_events (
			id text PRIMARY KEY,
			organization_id text NOT NULL,
			actor_type text NOT NULL,
			actor_id text NOT NULL,
			action text NOT NULL,
			resource_type text NOT NULL,
			resource_id text NOT NULL,
			result text NOT NULL,
			request_id text NOT NULL,
			metadata jsonb NOT NULL,
			created_at timestamptz NOT NULL
		)`,
	}); err != nil {
		t.Fatalf("create audit schema: %v", err)
	}

	pool := &pgxpooladapter.Adapter{Pool: h.Pool}
	store := New(pool, Options{Clock: h.FixedTime})
	event := audit.Event{
		ID:        h.NextText("audit"),
		TenantID:  "org-test",
		Actor:     audit.Actor{Type: "service", ID: "worker-test"},
		Action:    "widget.created",
		Resource:  audit.Resource{Type: "widget", ID: "widget-test"},
		Result:    audit.ResultSuccess,
		RequestID: "request-test",
		Metadata:  map[string]string{"fixture": h.NextText("metadata")},
	}
	if err := store.Record(ctx, event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	var metadata string
	var occurredAt time.Time
	if err := h.Pool.QueryRow(ctx, "SELECT metadata::text, created_at FROM audit_events WHERE id = $1", event.ID).Scan(&metadata, &occurredAt); err != nil {
		t.Fatalf("read recorded event: %v", err)
	}
	if metadata == "" || !occurredAt.Equal(h.FixedTime()) {
		t.Fatalf("stored audit event = metadata %q at %v", metadata, occurredAt)
	}
	if err := store.Record(ctx, event); err == nil {
		t.Fatal("duplicate audit event was accepted")
	}

	rollbackID := h.NextText("audit")
	manager := txpostgres.New(pool)
	rollback := errors.New("rollback fixture")
	err := manager.WithinTx(ctx, func(txCtx context.Context) error {
		rollbackEvent := event
		rollbackEvent.ID = rollbackID
		if err := store.Record(txCtx, rollbackEvent); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTx() error = %v, want rollback sentinel", err)
	}
	var rolledBackCount int
	if err := h.Pool.QueryRow(ctx, "SELECT count(*) FROM audit_events WHERE id = $1", rollbackID).Scan(&rolledBackCount); err != nil {
		t.Fatalf("check rolled-back audit event: %v", err)
	}
	if rolledBackCount != 0 {
		t.Fatalf("rolled-back audit event count = %d, want 0", rolledBackCount)
	}
	if err := store.Record(h.CanceledContext(t), audit.Event{ID: rollbackID, TenantID: "org-test", Actor: event.Actor, Action: event.Action, Resource: event.Resource, Result: event.Result, RequestID: event.RequestID}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Record() canceled context error = %v", err)
	}
}
