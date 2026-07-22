//go:build postgres

package outboxpostgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestStoreLeasesAndCompletesEventsAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "outbox-events",
		SQL: `CREATE TABLE outbox_events (
			id text PRIMARY KEY,
			organization_id text NOT NULL,
			event_type text NOT NULL,
			payload bytea NOT NULL,
			state text NOT NULL,
			next_at timestamptz NOT NULL,
			created_at timestamptz NOT NULL,
			lease_owner text,
			lease_expires_at timestamptz,
			retry_count integer NOT NULL DEFAULT 0
		)`,
	}); err != nil {
		t.Fatalf("create outbox schema: %v", err)
	}

	pool := &pgxpooladapter.Adapter{Pool: h.Pool}
	store := New(pool, Options{Clock: h.FixedTime, LeaseOwner: "worker-a", RetryDelay: time.Second})
	id := h.NextText("event")
	event := Event{ID: id, TenantID: "org-test", Type: "widget.created", Payload: []byte(`{"id":"widget-test"}`)}
	if err := store.Enqueue(ctx, event); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if err := store.Enqueue(ctx, event); err == nil {
		t.Fatal("duplicate outbox event was accepted")
	}
	jobs, err := store.Lease(ctx, 1)
	if err != nil || len(jobs) != 1 || jobs[0].ID != id {
		t.Fatalf("Lease() = (%#v, %v)", jobs, err)
	}
	if err := store.Complete(ctx, id); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if jobs, err := store.Lease(ctx, 1); err != nil || len(jobs) != 0 {
		t.Fatalf("Lease() after complete = (%#v, %v)", jobs, err)
	}
	if err := store.Complete(ctx, id); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("second Complete() error = %v", err)
	}
	if err := store.Enqueue(h.CanceledContext(t), Event{ID: h.NextText("event"), TenantID: "org-test", Type: "widget.created", Payload: []byte("{}")}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Enqueue() canceled context error = %v", err)
	}
}

func TestStoreConcurrentLeasesHaveOneConsumerAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "outbox-events",
		SQL: `CREATE TABLE outbox_events (
			id text PRIMARY KEY, organization_id text NOT NULL, event_type text NOT NULL,
			payload bytea NOT NULL, state text NOT NULL, next_at timestamptz NOT NULL,
			created_at timestamptz NOT NULL, lease_owner text, lease_expires_at timestamptz,
			retry_count integer NOT NULL DEFAULT 0
		)`,
	}); err != nil {
		t.Fatalf("create outbox schema: %v", err)
	}
	pool := &pgxpooladapter.Adapter{Pool: h.Pool}
	first := New(pool, Options{Clock: h.FixedTime, LeaseOwner: "worker-a"})
	second := New(pool, Options{Clock: h.FixedTime, LeaseOwner: "worker-b"})
	if err := first.Enqueue(ctx, Event{ID: h.NextText("event"), TenantID: "org-test", Type: "widget.created", Payload: []byte("{}")}); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, store := range []*Store{first, second} {
		wg.Add(1)
		go func(store *Store) {
			defer wg.Done()
			<-start
			jobs, err := store.Lease(ctx, 1)
			if err != nil {
				results <- -1
				return
			}
			results <- len(jobs)
		}(store)
	}
	close(start)
	wg.Wait()
	close(results)
	total := 0
	for got := range results {
		if got < 0 {
			t.Fatal("concurrent Lease() returned an error")
		}
		total += got
	}
	if total != 1 {
		t.Fatalf("concurrent leased jobs = %d, want 1", total)
	}
}
