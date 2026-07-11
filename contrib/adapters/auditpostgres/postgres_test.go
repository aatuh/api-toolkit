package auditpostgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/audit"
	"github.com/aatuh/api-toolkit/contrib/v4/audit/audittest"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

func TestStoreContract(t *testing.T) {
	audittest.AssertRecorderContract(t, func(t testing.TB) audit.Recorder {
		t.Helper()
		return New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	})
}

func TestRecordInsertsExpectedColumns(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeDBConnection{}
	store := New(&fakeDBPool{conn: conn}, Options{
		Table: "app.audit_events",
		Clock: func() time.Time {
			return now
		},
	})
	event := validEvent()
	event.OccurredAt = time.Time{}
	event.Metadata = map[string]string{"plan": "pro"}

	if err := store.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if len(conn.execCalls) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(conn.execCalls))
	}
	call := conn.execCalls[0]
	if strings.Contains(call.sql, "// #nosec") {
		t.Fatalf("Exec() SQL leaked Go comment text: %s", call.sql)
	}
	for _, want := range []string{
		`insert into "app"."audit_events"`,
		"organization_id",
		"actor_type",
		"metadata",
	} {
		if !strings.Contains(call.sql, want) {
			t.Fatalf("Exec() SQL missing %q: %s", want, call.sql)
		}
	}
	wantArgs := []any{
		"audit_01",
		"org_01",
		"user",
		"usr_01",
		"widget.create",
		"widget",
		"wgt_01",
		"success",
		"req_01",
		now,
	}
	for i, want := range wantArgs[:9] {
		if got := call.args[i]; got != want {
			t.Fatalf("Exec() arg[%d] = %#v, want %#v", i, got, want)
		}
	}
	var metadata map[string]string
	if err := json.Unmarshal(call.args[9].([]byte), &metadata); err != nil {
		t.Fatalf("metadata JSON: %v", err)
	}
	if got := metadata["plan"]; got != "pro" {
		t.Fatalf("metadata[plan] = %q, want pro", got)
	}
	if got := call.args[10]; got != now {
		t.Fatalf("Exec() arg[10] = %#v, want %#v", got, now)
	}
}

func TestRecordRejectsInvalidTable(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{Table: "audit_events;drop"})
	if err := store.Record(context.Background(), validEvent()); !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("Record() error = %v, want %v", err, ErrInvalidTable)
	}
}

func TestRecordUsesTransactionFromContext(t *testing.T) {
	t.Parallel()

	tx := &fakeDBTransaction{}
	conn := &fakeDBConnection{tx: tx}
	pool := &fakeDBPool{conn: conn}
	store := New(pool, Options{})
	manager := txpostgres.New(pool)

	err := manager.WithinTx(context.Background(), func(ctx context.Context) error {
		return store.Record(ctx, validEvent())
	})
	if err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	if pool.acquireCount != 1 {
		t.Fatalf("Acquire() calls = %d, want 1", pool.acquireCount)
	}
	if len(conn.execCalls) != 0 {
		t.Fatalf("pooled connection Exec() calls = %d, want 0", len(conn.execCalls))
	}
	if len(tx.execCalls) != 1 {
		t.Fatalf("transaction Exec() calls = %d, want 1", len(tx.execCalls))
	}
	if tx.commitCount != 1 {
		t.Fatalf("Commit() calls = %d, want 1", tx.commitCount)
	}
	if tx.rollbackCount != 1 {
		t.Fatalf("Rollback() calls = %d, want 1", tx.rollbackCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
}

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{}, Options{})
	result := store.HealthChecker().Check(context.Background())
	if result.Status != health.StatusHealthy || result.Message != "postgres audit healthy" {
		t.Fatalf("healthy result = %#v", result)
	}
	failed := New(&fakeDBPool{pingErr: errors.New("down")}, Options{})
	result = failed.HealthChecker().Check(context.Background())
	if result.Status != health.StatusUnhealthy || !strings.Contains(result.Message, "postgres audit ping failed") {
		t.Fatalf("unhealthy result = %#v", result)
	}
	if result := (*Store)(nil).HealthChecker().Check(context.Background()); result.Status != health.StatusUnhealthy {
		t.Fatalf("nil store result = %#v", result)
	}
}

func validEvent() audit.Event {
	return audit.Event{
		ID:       "audit_01",
		TenantID: "org_01",
		Actor: audit.Actor{
			Type: "user",
			ID:   "usr_01",
		},
		Action: "widget.create",
		Resource: audit.Resource{
			Type: "widget",
			ID:   "wgt_01",
		},
		Result:    audit.ResultSuccess,
		RequestID: "req_01",
	}
}

type fakeDBPool struct {
	conn         contracts.DatabaseConnection
	acquireErr   error
	pingErr      error
	acquireCount int
}

func (p *fakeDBPool) Ping(context.Context) error { return p.pingErr }
func (p *fakeDBPool) Close()                     {}

func (p *fakeDBPool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
	p.acquireCount++
	if p.acquireErr != nil {
		return nil, p.acquireErr
	}
	return p.conn, nil
}

type execCall struct {
	sql  string
	args []any
}

type fakeDBConnection struct {
	execCalls    []execCall
	tx           contracts.DatabaseTransaction
	releaseCount int
}

func (c *fakeDBConnection) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (c *fakeDBConnection) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	return scanFuncRow(func(...any) error { return nil })
}

func (c *fakeDBConnection) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(1), nil
}

func (c *fakeDBConnection) Begin(context.Context) (contracts.DatabaseTransaction, error) {
	return c.tx, nil
}

func (c *fakeDBConnection) Release() {
	c.releaseCount++
}

type fakeDBTransaction struct {
	execCalls     []execCall
	commitCount   int
	rollbackCount int
}

func (t *fakeDBTransaction) Query(context.Context, string, ...any) (contracts.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (t *fakeDBTransaction) QueryRow(context.Context, string, ...any) contracts.DatabaseRow {
	return scanFuncRow(func(...any) error { return nil })
}

func (t *fakeDBTransaction) Exec(_ context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	t.execCalls = append(t.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(1), nil
}

func (t *fakeDBTransaction) Commit(context.Context) error {
	t.commitCount++
	return nil
}

func (t *fakeDBTransaction) Rollback(context.Context) error {
	t.rollbackCount++
	return nil
}

type fakeDBRows struct{}

func (r fakeDBRows) Next() bool                 { return false }
func (r fakeDBRows) Scan(...any) error          { return nil }
func (r fakeDBRows) Close()                     {}
func (r fakeDBRows) Err() error                 { return nil }
func (r fakeDBRows) CommandTag() map[string]any { return nil }

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type scanFuncRow func(...any) error

func (r scanFuncRow) Scan(dest ...any) error { return r(dest...) }
