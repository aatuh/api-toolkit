package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/auditpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/migrate"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/operationpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/outboxpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/audit"
	integrationpgxpool "github.com/aatuh/api-toolkit/contrib/v4/integrations/pgxpool"
	integrationtxpostgres "github.com/aatuh/api-toolkit/contrib/v4/integrations/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/migrator"
	schedulerpostgres "github.com/aatuh/api-toolkit/contrib/v4/scheduler/postgres"
	"github.com/aatuh/api-toolkit/contrib/v4/testpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
	"github.com/aatuh/api-toolkit/v4/operations"
)

// TestRealPostgresSupportedAdapters proves the supported PostgreSQL
// adapters against the isolated PostgreSQL 18 harness rather than fake pools.
// Each subtest gets its own database, schema, and applied reference migrations.
func TestRealPostgresSupportedAdapters(t *testing.T) {
	requirePostgres(t)

	t.Run("migrations are applied and idempotent", testMigrations)
	t.Run("migration adapter applies the reference service schema", testMigrationAdapter)
	t.Run("pgx pool and transaction adapters", testPoolAndTransactions)
	t.Run("audit and operation adapters", testAuditAndOperations)
	t.Run("outbox adapter leases concurrent consumers", testOutbox)
	t.Run("webhook delivery adapter persists durable workflow", testWebhookDelivery)
	t.Run("scheduler storage persists and orders runs", testSchedulerStorage)
}

func requirePostgres(t *testing.T) {
	t.Helper()
	if os.Getenv(testpostgres.EnableEnv) != "1" {
		t.Skipf("requires %s=1; run make supported-adapter-check", testpostgres.EnableEnv)
	}
}

func testMigrations(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()

	for _, table := range []string{
		"organizations", "operations", "outbox_events", "audit_events",
		"webhook_endpoints", "webhook_deliveries",
	} {
		var found bool
		if err := h.Pool().QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&found); err != nil {
			t.Fatalf("look up migrated table %q: %v", table, err)
		}
		if !found {
			t.Fatalf("migration did not create %q", table)
		}
	}

	if err := h.ApplyMigrations(ctx, migrator.Options{MigrationsDirs: []string{referenceMigrationsDir(t)}}); err != nil {
		t.Fatalf("apply reference migrations a second time: %v", err)
	}
}

func testMigrationAdapter(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	adapter, err := migrate.NewWithContext(ctx, migrate.Options{
		DSN:  h.DSN(),
		Dirs: []string{referenceMigrationsDir(t)},
		Log:  discardLogger{},
	})
	if err != nil {
		t.Fatalf("construct real migration adapter: %v", err)
	}
	t.Cleanup(func() {
		if err := adapter.Close(); err != nil {
			t.Errorf("close real migration adapter: %v", err)
		}
	})
	if err := adapter.Up(ctx, ""); err != nil {
		t.Fatalf("apply reference migrations through adapter: %v", err)
	}
	status, err := adapter.Status(ctx, "")
	if err != nil {
		t.Fatalf("read real migration adapter status: %v", err)
	}
	if status == "" {
		t.Fatal("real migration adapter status was empty")
	}
	var tableCount int
	if err := h.Pool().QueryRow(ctx,
		"SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('organizations', 'operations', 'outbox_events')",
	).Scan(&tableCount); err != nil {
		t.Fatalf("verify migration adapter schema: %v", err)
	}
	if tableCount != 3 {
		t.Fatalf("migration adapter created %d expected reference tables, want 3", tableCount)
	}
}

func testPoolAndTransactions(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()
	pool := h.Adapter()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("real pgx pool Ping(): %v", err)
	}
	legacyPool, err := integrationpgxpool.New(h.DSN())
	if err != nil {
		t.Fatalf("construct real pgxpool integration: %v", err)
	}
	t.Cleanup(legacyPool.Close)
	if err := legacyPool.Ping(ctx); err != nil {
		t.Fatalf("real pgxpool integration Ping(): %v", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("real pgx pool Acquire(): %v", err)
	}
	defer conn.Release()

	var value int
	if err := conn.QueryRow(ctx, "SELECT 42").Scan(&value); err != nil {
		t.Fatalf("real pgx pool QueryRow(): %v", err)
	}
	if value != 42 {
		t.Fatalf("real pgx pool value = %d, want 42", value)
	}

	cancelCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := conn.Exec(cancelCtx, "SELECT pg_sleep(1)"); err == nil {
		t.Fatal("real pgx pool cancellation returned nil error")
	} else if !errors.Is(cancelCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("real pgx pool cancellation context = %v, want deadline exceeded", cancelCtx.Err())
	}

	manager := txpostgres.New(pool)
	if err := manager.WithinTx(ctx, func(txCtx context.Context) error {
		_, err := txpostgres.FromCtx(txCtx, pool).Exec(txCtx,
			"INSERT INTO audit_events (id, actor_type, action, resource_type, result, metadata) VALUES ($1, $2, $3, $4, $5, $6)",
			h.FixtureID("committed-audit"), "system", "transaction.commit", "test", "success", []byte("{}"),
		)
		return err
	}); err != nil {
		t.Fatalf("transaction commit: %v", err)
	}

	rollbackID := h.FixtureID("rolled-back-audit")
	rollbackErr := errors.New("force rollback")
	err = manager.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := txpostgres.FromCtx(txCtx, pool).Exec(txCtx,
			"INSERT INTO audit_events (id, actor_type, action, resource_type, result, metadata) VALUES ($1, $2, $3, $4, $5, $6)",
			rollbackID, "system", "transaction.rollback", "test", "failure", []byte("{}"),
		); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction rollback error = %v, want %v", err, rollbackErr)
	}
	var exists bool
	if err := h.Pool().QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM audit_events WHERE id=$1)", rollbackID).Scan(&exists); err != nil {
		t.Fatalf("verify rollback: %v", err)
	}
	if exists {
		t.Fatal("rollback transaction left a durable row")
	}
	if err := integrationtxpostgres.New(pool).WithinTx(ctx, func(txCtx context.Context) error {
		_, err := integrationtxpostgres.FromCtx(txCtx, pool).Exec(txCtx, "SELECT 1")
		return err
	}); err != nil {
		t.Fatalf("real txpostgres integration: %v", err)
	}

	if err := h.InterruptConnections(ctx); err != nil {
		t.Fatalf("interrupt isolated database connections: %v", err)
	}
	recovered := false
	for attempt := 0; attempt < 5; attempt++ {
		if err := pool.Ping(ctx); err == nil {
			recovered = true
			break
		}
	}
	if !recovered {
		t.Fatal("pool did not recover after isolated connection loss")
	}
}

func testAuditAndOperations(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()
	pool := h.Adapter()
	tenantID := h.FixtureID("tenant")
	seedOrganization(t, h, tenantID)

	event := audit.Event{
		ID:       h.FixtureID("audit"),
		TenantID: tenantID,
		Actor:    audit.Actor{Type: "service", ID: h.FixtureID("actor")},
		Action:   "widget.created",
		Resource: audit.Resource{Type: "widget", ID: h.FixtureID("widget")},
		Result:   audit.ResultSuccess,
		Metadata: map[string]string{"fixture": "real-postgres"},
	}
	auditStore := auditpostgres.New(pool, auditpostgres.Options{})
	if err := auditStore.Record(ctx, event); err != nil {
		t.Fatalf("record audit event: %v", err)
	}
	if err := auditStore.Record(ctx, event); err == nil {
		t.Fatal("duplicate audit event returned nil error")
	}
	var metadata []byte
	if err := h.Pool().QueryRow(ctx, "SELECT metadata FROM audit_events WHERE id=$1", event.ID).Scan(&metadata); err != nil {
		t.Fatalf("load stored audit metadata: %v", err)
	}
	var gotMetadata map[string]string
	if err := json.Unmarshal(metadata, &gotMetadata); err != nil {
		t.Fatalf("decode stored audit metadata: %v", err)
	}
	if gotMetadata["fixture"] != "real-postgres" {
		t.Fatalf("stored audit metadata = %#v", gotMetadata)
	}

	type result struct {
		Message string `json:"message"`
	}
	operationStore := operationpostgres.New[result](pool, operationpostgres.Options{})
	operationCtx := operationpostgres.WithTenantID(ctx, tenantID)
	operationID := h.FixtureID("operation")
	if err := operationStore.CreateOperation(operationCtx, operations.Operation[result]{
		ID: operationID, State: operations.StatePending,
	}); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	wantResult := result{Message: "complete"}
	if err := operationStore.UpdateOperation(operationCtx, operations.Operation[result]{
		ID: operationID, State: operations.StateSucceeded, Result: &wantResult,
	}); err != nil {
		t.Fatalf("update operation: %v", err)
	}
	got, ok, err := operationStore.GetOperation(operationCtx, operationID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if !ok || got.State != operations.StateSucceeded || got.Result == nil || got.Result.Message != wantResult.Message {
		t.Fatalf("stored operation = %#v, found=%t", got, ok)
	}
	if _, ok, err := operationStore.GetOperation(operationCtx, h.FixtureID("missing-operation")); err != nil || ok {
		t.Fatalf("missing operation = found=%t, error=%v", ok, err)
	}
	if err := operationStore.CreateOperation(context.Background(), operations.Operation[result]{ID: h.FixtureID("unscoped")}); !errors.Is(err, operationpostgres.ErrTenantRequired) {
		t.Fatalf("unscoped create error = %v, want tenant requirement", err)
	}
}

func testOutbox(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()
	pool := h.Adapter()
	tenantID := h.FixtureID("tenant")
	seedOrganization(t, h, tenantID)
	now := time.Now().UTC().Truncate(time.Microsecond)

	store := outboxpostgres.New(pool, outboxpostgres.Options{
		Clock:      func() time.Time { return now },
		LeaseOwner: "worker-primary",
	})
	event := outboxpostgres.Event{
		ID: h.FixtureID("outbox"), TenantID: tenantID, Type: "widget.created", Payload: []byte(`{"id":"fixture"}`), NextAt: now,
	}
	if err := store.Enqueue(ctx, event); err != nil {
		t.Fatalf("enqueue outbox event: %v", err)
	}
	if err := store.Enqueue(ctx, event); err == nil {
		t.Fatal("duplicate outbox event returned nil error")
	}
	jobs, err := store.Lease(ctx, 1)
	if err != nil {
		t.Fatalf("lease outbox event: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != event.ID || jobs[0].TenantID != tenantID {
		t.Fatalf("leased jobs = %#v", jobs)
	}
	if err := store.Complete(ctx, event.ID); err != nil {
		t.Fatalf("complete outbox event: %v", err)
	}
	var state string
	if err := h.Pool().QueryRow(ctx, "SELECT state FROM outbox_events WHERE id=$1", event.ID).Scan(&state); err != nil {
		t.Fatalf("load completed outbox event: %v", err)
	}
	if state != "succeeded" {
		t.Fatalf("completed outbox state = %q, want succeeded", state)
	}

	for _, label := range []string{"concurrent-a", "concurrent-b"} {
		if err := store.Enqueue(ctx, outboxpostgres.Event{
			ID: h.FixtureID(label), TenantID: tenantID, Type: "widget.updated", Payload: []byte(`{"id":"fixture"}`), NextAt: now,
		}); err != nil {
			t.Fatalf("enqueue %s: %v", label, err)
		}
	}

	stores := []*outboxpostgres.Store{
		outboxpostgres.New(pool, outboxpostgres.Options{Clock: func() time.Time { return now }, LeaseOwner: "worker-a"}),
		outboxpostgres.New(pool, outboxpostgres.Options{Clock: func() time.Time { return now }, LeaseOwner: "worker-b"}),
	}
	type leaseResult struct {
		jobs []string
		err  error
	}
	results := make(chan leaseResult, len(stores))
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, concurrentStore := range stores {
		workers.Add(1)
		go func(concurrentStore *outboxpostgres.Store) {
			defer workers.Done()
			<-start
			jobs, err := concurrentStore.Lease(ctx, 1)
			ids := make([]string, 0, len(jobs))
			for _, job := range jobs {
				ids = append(ids, job.ID)
			}
			results <- leaseResult{jobs: ids, err: err}
		}(concurrentStore)
	}
	close(start)
	workers.Wait()
	close(results)
	leased := map[string]bool{}
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent lease: %v", result.err)
		}
		for _, id := range result.jobs {
			if leased[id] {
				t.Fatalf("concurrent consumers leased %q twice", id)
			}
			leased[id] = true
		}
	}
	if len(leased) != 2 {
		t.Fatalf("concurrent consumers leased %d events, want 2", len(leased))
	}
}

func testWebhookDelivery(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()
	pool := h.Adapter()
	tenantID := h.FixtureID("tenant")
	seedOrganization(t, h, tenantID)
	now := time.Now().UTC().Truncate(time.Microsecond)
	endpointID := h.FixtureID("endpoint")
	if _, err := h.Pool().Exec(ctx,
		"INSERT INTO webhook_endpoints (id, organization_id, url, event_types, secret_hash, secret_ciphertext, secret_nonce, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)",
		endpointID, tenantID, "https://example.invalid/hooks", []string{"widget.created"}, []byte("hash"), []byte("ciphertext"), []byte("nonce"), now,
	); err != nil {
		t.Fatalf("seed webhook endpoint: %v", err)
	}
	store := webhookdeliverypostgres.New(pool, webhookdeliverypostgres.Options{
		Clock: func() time.Time { return now },
		SecretResolver: webhookdeliverypostgres.SecretResolverFunc(func(_ context.Context, gotTenantID, gotEndpointID string) ([]byte, bool, error) {
			if gotTenantID != tenantID || gotEndpointID != endpointID {
				return nil, false, fmt.Errorf("unexpected endpoint lookup")
			}
			return []byte("fixture-signing-material"), true, nil
		}),
	})
	endpoints, err := store.ListEndpoints(ctx, tenantID, "widget.created")
	if err != nil {
		t.Fatalf("list webhook endpoints: %v", err)
	}
	if len(endpoints) != 1 || endpoints[0].ID != endpointID || len(endpoints[0].SigningSecret) == 0 {
		gotID := ""
		secretLength := 0
		if len(endpoints) > 0 {
			gotID = endpoints[0].ID
			secretLength = len(endpoints[0].SigningSecret)
		}
		t.Fatalf("listed endpoints count=%d first_id=%q signing_secret_length=%d", len(endpoints), gotID, secretLength)
	}
	if _, ok, err := store.GetEndpoint(ctx, h.FixtureID("other-tenant"), endpointID); err != nil || ok {
		t.Fatalf("cross-tenant endpoint read = found=%t, error=%v", ok, err)
	}

	deliveryID := h.FixtureID("delivery")
	event := webhookdelivery.Event{ID: h.FixtureID("event"), TenantID: tenantID, Type: "widget.created", Payload: json.RawMessage(`{"id":"fixture"}`), OccurredAt: now}
	delivery := webhookdelivery.Delivery{
		ID: deliveryID, TenantID: tenantID, EndpointID: endpointID, EventID: event.ID, EventType: event.Type,
		URL: endpoints[0].URL, State: webhookdelivery.StatePending, Attempt: 0, NextAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.EnqueueDelivery(ctx, delivery, webhookdelivery.JobPayload{DeliveryID: deliveryID, EndpointID: endpointID, Event: event}); err != nil {
		t.Fatalf("enqueue webhook delivery: %v", err)
	}
	if err := store.RecordAttempt(ctx, webhookdelivery.AttemptResult{
		DeliveryID: deliveryID, TenantID: tenantID, EndpointID: endpointID, EventID: event.ID, EventType: event.Type,
		Attempt: 1, StatusCode: 503, Retryable: true, Error: "token fixture value must not persist", OccurredAt: now,
	}); err != nil {
		t.Fatalf("record webhook attempt: %v", err)
	}
	var lastError string
	if err := h.Pool().QueryRow(ctx, "SELECT last_error FROM webhook_deliveries WHERE id=$1", deliveryID).Scan(&lastError); err != nil {
		t.Fatalf("load webhook attempt error: %v", err)
	}
	if lastError != "webhook delivery failed" {
		t.Fatalf("stored webhook failure = %q", lastError)
	}
	replayAt := now.Add(time.Minute)
	if err := store.ReplayDelivery(ctx, tenantID, deliveryID, replayAt); err != nil {
		t.Fatalf("replay webhook delivery: %v", err)
	}
	var outboxType string
	if err := h.Pool().QueryRow(ctx, "SELECT event_type FROM outbox_events WHERE id=$1", deliveryID).Scan(&outboxType); err != nil {
		t.Fatalf("load webhook outbox event: %v", err)
	}
	if outboxType != webhookdeliverypostgres.OutboxEventType {
		t.Fatalf("webhook outbox type = %q, want %q", outboxType, webhookdeliverypostgres.OutboxEventType)
	}
}

func testSchedulerStorage(t *testing.T) {
	h := migratedHarness(t)
	ctx := context.Background()
	if _, err := h.Pool().Exec(ctx, `
		CREATE TABLE scheduler_runs (
			job_name TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ NOT NULL,
			success BOOLEAN NOT NULL,
			error TEXT NOT NULL
		)`); err != nil {
		t.Fatalf("create scheduler storage: %v", err)
	}
	repo := schedulerpostgres.NewRunsRepo(h.Adapter())
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	firstFinished := startedAt.Add(time.Second)
	secondFinished := startedAt.Add(2 * time.Second)
	if err := repo.Record(ctx, "fixture-job", startedAt, firstFinished, true, ""); err != nil {
		t.Fatalf("record first scheduler run: %v", err)
	}
	if err := repo.Record(ctx, "fixture-job", startedAt, secondFinished, false, "failure"); err != nil {
		t.Fatalf("record second scheduler run: %v", err)
	}
	got, ok, err := repo.LastFinished(ctx, "fixture-job")
	if err != nil || !ok || !got.Equal(secondFinished) {
		t.Fatalf("last scheduler run = %v, found=%t, error=%v", got, ok, err)
	}
	if _, ok, err := repo.LastFinished(ctx, "missing-job"); err != nil || ok {
		t.Fatalf("missing scheduler run = found=%t, error=%v", ok, err)
	}
}

func migratedHarness(t *testing.T) *testpostgres.Harness {
	t.Helper()
	h := testpostgres.New(t)
	if err := h.ApplyMigrations(context.Background(), migrator.Options{MigrationsDirs: []string{referenceMigrationsDir(t)}}); err != nil {
		t.Fatalf("apply reference service migrations: %v", err)
	}
	return h
}

func referenceMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate real PostgreSQL test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
}

func seedOrganization(t *testing.T, h *testpostgres.Harness, organizationID string) {
	t.Helper()
	if _, err := h.Pool().Exec(context.Background(),
		"INSERT INTO organizations (id, name) VALUES ($1, $2)", organizationID, "Real PostgreSQL fixture",
	); err != nil {
		t.Fatalf("seed organization: %v", err)
	}
}

type discardLogger struct{}

func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}
