package outboxpostgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/async"
	"github.com/aatuh/api-toolkit/contrib/v2/async/asynctest"
	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestAsyncStoreContract(t *testing.T) {
	asynctest.AssertStoreContract(t, func(testing.TB) async.Store {
		return New(&fakeDBPool{conn: &fakeDBConnection{
			rows: &fakeRows{values: [][]any{
				{"evt_1", "org_1", "widget.created", []byte(`{"id":"wgt_1"}`), 0},
			}},
			rowsAffected: 1,
		}}, Options{})
	})
}

func TestEnqueueInsertsPendingEvent(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store := New(&fakeDBPool{conn: conn}, Options{
		Table: "app.outbox_events",
		Clock: func() time.Time { return now },
	})
	err := store.Enqueue(context.Background(), Event{
		ID:       "evt_1",
		TenantID: "org_1",
		Type:     "widget.created",
		Payload:  []byte(`{"id":"wgt_1"}`),
	})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, `insert into "app"."outbox_events"`) {
		t.Fatalf("Exec() SQL = %s", call.sql)
	}
	wantArgs := []any{"evt_1", "org_1", "widget.created"}
	for i, want := range wantArgs {
		if got := call.args[i]; got != want {
			t.Fatalf("Exec() arg[%d] = %#v, want %#v", i, got, want)
		}
	}
	if got := string(call.args[3].([]byte)); got != `{"id":"wgt_1"}` {
		t.Fatalf("payload arg = %q", got)
	}
	if got := call.args[4]; got != now {
		t.Fatalf("next_at arg = %#v, want %#v", got, now)
	}
}

func TestEnqueueRejectsInvalidEvent(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{})
	err := store.Enqueue(context.Background(), Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created"})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Enqueue() error = %v, want %v", err, ErrInvalidEvent)
	}
}

func TestLeaseReturnsAsyncJobs(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0).UTC()
	conn := &fakeDBConnection{
		rows: &fakeRows{
			values: [][]any{
				{"evt_1", "org_1", "widget.created", []byte(`{"id":"wgt_1"}`), 2},
				{"evt_2", "org_2", "widget.deleted", []byte(`{"id":"wgt_2"}`), 0},
			},
		},
	}
	store := New(&fakeDBPool{conn: conn}, Options{
		LeaseOwner:    "worker-a",
		LeaseDuration: 2 * time.Minute,
		Clock:         func() time.Time { return now },
	})
	jobs, err := store.Lease(context.Background(), 10)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs len = %d, want 2", len(jobs))
	}
	if jobs[0].ID != "evt_1" || jobs[0].TenantID != "org_1" || jobs[0].Kind != "widget.created" || jobs[0].Attempts != 2 {
		t.Fatalf("job[0] = %#v", jobs[0])
	}
	if string(jobs[0].Payload) != `{"id":"wgt_1"}` {
		t.Fatalf("job[0] payload = %q", jobs[0].Payload)
	}
	if !strings.Contains(conn.querySQL, "for update skip locked") {
		t.Fatalf("Lease() SQL missing skip locked: %s", conn.querySQL)
	}
	if got := conn.queryArgs; len(got) != 4 || got[0] != "worker-a" || got[1] != now.Add(2*time.Minute) || got[2] != now || got[3] != 10 {
		t.Fatalf("Lease() args = %#v", got)
	}
	if !conn.rows.closed {
		t.Fatal("rows closed = false, want true")
	}
}

func TestLeaseReturnsNilForNonPositiveLimit(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{}
	store := New(&fakeDBPool{conn: conn}, Options{})
	jobs, err := store.Lease(context.Background(), 0)
	if err != nil {
		t.Fatalf("Lease() error = %v", err)
	}
	if jobs != nil {
		t.Fatalf("jobs = %#v, want nil", jobs)
	}
	if conn.querySQL != "" {
		t.Fatalf("Query() SQL = %q, want empty", conn.querySQL)
	}
}

func TestCompleteAndFailRequireCurrentLeaseOwner(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_100, 0).UTC()
	conn := &fakeDBConnection{rowsAffected: 1}
	store := New(&fakeDBPool{conn: conn}, Options{
		LeaseOwner:  "worker-a",
		RetryDelay:  5 * time.Second,
		MaxAttempts: 3,
		Clock:       func() time.Time { return now },
	})
	if err := store.Complete(context.Background(), "evt_1"); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := store.Fail(context.Background(), "evt_2", "ignored secret detail"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	complete := conn.execCalls[0]
	if !strings.Contains(complete.sql, "lease_owner=$2") {
		t.Fatalf("Complete() SQL = %s", complete.sql)
	}
	if got := complete.args; len(got) != 2 || got[0] != "evt_1" || got[1] != "worker-a" {
		t.Fatalf("Complete() args = %#v", got)
	}
	failed := conn.execCalls[1]
	if strings.Contains(failed.sql, "ignored secret") {
		t.Fatalf("Fail() SQL leaked message: %s", failed.sql)
	}
	if !strings.Contains(failed.sql, "$3::timestamptz") {
		t.Fatalf("Fail() SQL should cast retry base timestamp: %s", failed.sql)
	}
	if got := failed.args; len(got) != 6 || got[0] != "evt_2" || got[1] != "worker-a" || got[2] != now || got[3] != 3 || got[4] != float64(5) || got[5] != float64(60) {
		t.Fatalf("Fail() args = %#v", got)
	}
}

func TestCompleteReportsMissingEvent(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{rowsAffected: 0}}, Options{})
	err := store.Complete(context.Background(), "missing")
	if !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("Complete() error = %v, want %v", err, ErrEventNotFound)
	}
}

func TestInvalidTableRejected(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{conn: &fakeDBConnection{}}, Options{Table: "outbox;drop"})
	err := store.Enqueue(context.Background(), Event{ID: "evt_1", TenantID: "org_1", Type: "widget.created", Payload: []byte("{}")})
	if !errors.Is(err, ErrInvalidTable) {
		t.Fatalf("Enqueue() error = %v, want %v", err, ErrInvalidTable)
	}
}

func TestHealthChecker(t *testing.T) {
	t.Parallel()

	store := New(&fakeDBPool{}, Options{})
	result := store.HealthChecker().Check(context.Background())
	if result.Status != ports.HealthStatusHealthy || result.Message != "postgres outbox healthy" {
		t.Fatalf("healthy result = %#v", result)
	}
	failed := New(&fakeDBPool{pingErr: errors.New("down")}, Options{})
	result = failed.HealthChecker().Check(context.Background())
	if result.Status != ports.HealthStatusUnhealthy || !strings.Contains(result.Message, "postgres outbox ping failed") {
		t.Fatalf("unhealthy result = %#v", result)
	}
	if result := (*Store)(nil).HealthChecker().Check(context.Background()); result.Status != ports.HealthStatusUnhealthy {
		t.Fatalf("nil store result = %#v", result)
	}
}

type fakeDBPool struct {
	conn    ports.DatabaseConnection
	pingErr error
}

func (p *fakeDBPool) Ping(context.Context) error { return p.pingErr }
func (p *fakeDBPool) Close()                     {}
func (p *fakeDBPool) Stat() ports.DatabaseStats  { return nil }

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return p.conn, nil
}

type execCall struct {
	sql  string
	args []any
}

type fakeDBConnection struct {
	execCalls    []execCall
	querySQL     string
	queryArgs    []any
	rows         *fakeRows
	rowsAffected int64
	releaseCount int
}

func (c *fakeDBConnection) Query(_ context.Context, sql string, args ...any) (ports.DatabaseRows, error) {
	c.querySQL = sql
	c.queryArgs = append([]any(nil), args...)
	if c.rows != nil {
		return c.rows, nil
	}
	return &fakeRows{}, nil
}

func (c *fakeDBConnection) QueryRow(context.Context, string, ...any) ports.DatabaseRow {
	return scanFuncRow(func(...any) error { return nil })
}

func (c *fakeDBConnection) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(c.rowsAffected), nil
}

func (c *fakeDBConnection) Begin(context.Context) (ports.DatabaseTransaction, error) {
	return nil, errors.New("transactions are not supported by fake connection")
}

func (c *fakeDBConnection) Release() {
	c.releaseCount++
}

type fakeRows struct {
	values [][]any
	index  int
	closed bool
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.values[r.index]
	r.index++
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *[]byte:
			*d = append([]byte(nil), row[i].([]byte)...)
		case *int:
			*d = row[i].(int)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return nil
}

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type scanFuncRow func(...any) error

func (r scanFuncRow) Scan(dest ...any) error { return r(dest...) }
