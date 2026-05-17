package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestRunsRepoRecordUsesPooledFacade(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 0).UTC()
	finishedAt := startedAt.Add(45 * time.Second)
	conn := &fakeDBConnection{}
	pool := &fakeDBPool{conn: conn}
	repo := NewRunsRepo(pool)

	err := repo.Record(context.Background(), "sync", startedAt, finishedAt, false, "boom")
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if pool.acquireCount != 1 {
		t.Fatalf("Acquire() calls = %d, want 1", pool.acquireCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if len(conn.execCalls) != 1 {
		t.Fatalf("Exec() calls = %d, want 1", len(conn.execCalls))
	}
	call := conn.execCalls[0]
	if !strings.Contains(call.sql, "insert into scheduler_runs") {
		t.Fatalf("Exec() SQL = %q", call.sql)
	}
	if got, want := call.args, []any{"sync", startedAt, finishedAt, false, "boom"}; len(got) != len(want) {
		t.Fatalf("Exec() args len = %d, want %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Exec() arg[%d] = %#v, want %#v", i, got[i], want[i])
			}
		}
	}
}

func TestRunsRepoLastFinishedReturnsTimestamp(t *testing.T) {
	t.Parallel()

	want := time.Unix(1_700_000_100, 0).UTC()
	conn := &fakeDBConnection{
		row: scanFuncRow(func(dest ...any) error {
			ts, ok := dest[0].(*time.Time)
			if !ok {
				t.Fatalf("Scan() destination type = %T, want *time.Time", dest[0])
			}
			*ts = want
			return nil
		}),
	}
	pool := &fakeDBPool{conn: conn}
	repo := NewRunsRepo(pool)

	got, ok, err := repo.LastFinished(context.Background(), "sync")
	if err != nil {
		t.Fatalf("LastFinished() error = %v", err)
	}
	if !ok {
		t.Fatal("LastFinished() ok = false, want true")
	}
	if !got.Equal(want) {
		t.Fatalf("LastFinished() = %v, want %v", got, want)
	}
	if pool.acquireCount != 1 {
		t.Fatalf("Acquire() calls = %d, want 1", pool.acquireCount)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if !strings.Contains(conn.queryRowSQL, "select finished_at") {
		t.Fatalf("QueryRow() SQL = %q", conn.queryRowSQL)
	}
}

func TestRunsRepoLastFinishedReturnsFalseForNoRows(t *testing.T) {
	t.Parallel()

	conn := &fakeDBConnection{
		row: scanFuncRow(func(...any) error {
			return pgx.ErrNoRows
		}),
	}
	pool := &fakeDBPool{conn: conn}
	repo := NewRunsRepo(pool)

	got, ok, err := repo.LastFinished(context.Background(), "missing")
	if err != nil {
		t.Fatalf("LastFinished() error = %v", err)
	}
	if ok {
		t.Fatal("LastFinished() ok = true, want false")
	}
	if !got.IsZero() {
		t.Fatalf("LastFinished() time = %v, want zero", got)
	}
	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
}

func TestRunsRepoRecordUsesTransactionFromContext(t *testing.T) {
	t.Parallel()

	tx := &fakeDBTransaction{}
	conn := &fakeDBConnection{tx: tx}
	pool := &fakeDBPool{conn: conn}
	repo := NewRunsRepo(pool)
	manager := txpostgres.New(pool)

	startedAt := time.Unix(1_700_000_200, 0).UTC()
	finishedAt := startedAt.Add(time.Minute)
	err := manager.WithinTx(context.Background(), func(ctx context.Context) error {
		return repo.Record(ctx, "sync", startedAt, finishedAt, true, "")
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

type fakeDBPool struct {
	conn         ports.DatabaseConnection
	acquireErr   error
	acquireCount int
}

func (p *fakeDBPool) Ping(context.Context) error { return nil }
func (p *fakeDBPool) Close()                     {}

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
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
	queryRowSQL  string
	queryRowArgs []any
	row          ports.DatabaseRow
	tx           ports.DatabaseTransaction
	releaseCount int
}

func (c *fakeDBConnection) Query(context.Context, string, ...any) (ports.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (c *fakeDBConnection) QueryRow(_ context.Context, sql string, args ...any) ports.DatabaseRow {
	c.queryRowSQL = sql
	c.queryRowArgs = append([]any(nil), args...)
	return c.row
}

func (c *fakeDBConnection) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	c.execCalls = append(c.execCalls, execCall{sql: sql, args: append([]any(nil), args...)})
	return fakeDBResult(1), nil
}

func (c *fakeDBConnection) Begin(context.Context) (ports.DatabaseTransaction, error) {
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

func (t *fakeDBTransaction) Query(context.Context, string, ...any) (ports.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (t *fakeDBTransaction) QueryRow(context.Context, string, ...any) ports.DatabaseRow {
	return nil
}

func (t *fakeDBTransaction) Exec(_ context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
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

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }

type fakeDBRows struct{}

func (fakeDBRows) Next() bool        { return false }
func (fakeDBRows) Scan(...any) error { return nil }
func (fakeDBRows) Close()            {}
func (fakeDBRows) Err() error        { return nil }

type scanFuncRow func(dest ...any) error

func (f scanFuncRow) Scan(dest ...any) error { return f(dest...) }
