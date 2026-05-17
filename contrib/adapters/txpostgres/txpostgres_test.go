package txpostgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestWithinTxUsesCleanupContextForRollback(t *testing.T) {
	type ctxKey string

	tests := []struct {
		name     string
		prepare  func(context.Context) (context.Context, context.CancelFunc)
		wantDone error
	}{
		{
			name: "canceled context",
			prepare: func(ctx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return ctx, func() {}
			},
			wantDone: context.Canceled,
		},
		{
			name: "timed out context",
			prepare: func(ctx context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(ctx, time.Nanosecond)
				for ctx.Err() == nil {
					time.Sleep(time.Millisecond)
				}
				return ctx, cancel
			},
			wantDone: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := ctxKey("request-id")
			baseCtx := context.WithValue(context.Background(), key, "req-123")
			ctx, cancel := tt.prepare(baseCtx)
			defer cancel()

			tx := &fakeDBTransaction{
				onRollback: func(ctx context.Context) {
					assertRollbackCleanupContext(t, ctx, key)
				},
			}
			conn := &fakeDBConnection{tx: tx}
			manager := New(&fakeDBPool{conn: conn})

			err := manager.WithinTx(ctx, func(ctx context.Context) error {
				if !errors.Is(ctx.Err(), tt.wantDone) {
					t.Fatalf("fn context error = %v, want %v", ctx.Err(), tt.wantDone)
				}
				if got := ctx.Value(key); got != "req-123" {
					t.Fatalf("fn context value = %v, want req-123", got)
				}
				return ctx.Err()
			})
			if !errors.Is(err, tt.wantDone) {
				t.Fatalf("WithinTx() error = %v, want %v", err, tt.wantDone)
			}
			if tx.commitCount != 0 {
				t.Fatalf("Commit() calls = %d, want 0", tx.commitCount)
			}
			if tx.rollbackCount != 1 {
				t.Fatalf("Rollback() calls = %d, want 1", tx.rollbackCount)
			}
			if conn.releaseCount != 1 {
				t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
			}
		})
	}
}

func TestPooledRowsCloseReleasesConnectionOnce(t *testing.T) {
	rows := &countingDBRows{}
	conn := &fakeDBConnection{rows: rows}
	db := FromCtx(context.Background(), &fakeDBPool{conn: conn})

	gotRows, err := db.Query(context.Background(), "select 1")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	gotRows.Close()
	gotRows.Close()

	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if rows.closeCount != 1 {
		t.Fatalf("rows Close() calls = %d, want 1", rows.closeCount)
	}
}

func TestPooledRowScanReleasesConnectionOnce(t *testing.T) {
	row := &countingDBRow{}
	conn := &fakeDBConnection{row: row}
	db := FromCtx(context.Background(), &fakeDBPool{conn: conn})

	gotRow := db.QueryRow(context.Background(), "select 1")
	if err := gotRow.Scan(); err != nil {
		t.Fatalf("first Scan() error = %v", err)
	}
	if err := gotRow.Scan(); err != nil {
		t.Fatalf("second Scan() error = %v", err)
	}

	if conn.releaseCount != 1 {
		t.Fatalf("Release() calls = %d, want 1", conn.releaseCount)
	}
	if row.scanCount != 2 {
		t.Fatalf("row Scan() calls = %d, want 2", row.scanCount)
	}
}

func assertRollbackCleanupContext[T comparable](
	t *testing.T, ctx context.Context, key T,
) {
	t.Helper()
	if ctx == nil {
		t.Fatal("expected rollback context")
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("rollback context error = %v, want nil", err)
	}
	if got := ctx.Value(key); got != "req-123" {
		t.Fatalf("rollback context value = %v, want req-123", got)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected rollback context deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatalf("rollback context deadline already elapsed: %v", remaining)
	}
	if remaining > rollbackCleanupTimeout+time.Second {
		t.Fatalf("rollback context deadline too far in future: %v", remaining)
	}
}

type fakeDBPool struct {
	conn ports.DatabaseConnection
}

func (p *fakeDBPool) Ping(context.Context) error { return nil }
func (p *fakeDBPool) Close()                     {}

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return p.conn, nil
}

type fakeDBConnection struct {
	tx           ports.DatabaseTransaction
	rows         ports.DatabaseRows
	row          ports.DatabaseRow
	releaseCount int
}

func (c *fakeDBConnection) Query(
	context.Context, string, ...any,
) (ports.DatabaseRows, error) {
	if c.rows != nil {
		return c.rows, nil
	}
	return fakeDBRows{}, nil
}

func (c *fakeDBConnection) QueryRow(
	context.Context, string, ...any,
) ports.DatabaseRow {
	if c.row != nil {
		return c.row
	}
	return fakeDBRow{}
}

func (c *fakeDBConnection) Exec(
	context.Context, string, ...any,
) (ports.DatabaseResult, error) {
	return fakeDBResult(0), nil
}

func (c *fakeDBConnection) Begin(
	context.Context,
) (ports.DatabaseTransaction, error) {
	return c.tx, nil
}

func (c *fakeDBConnection) Release() {
	c.releaseCount++
}

type fakeDBTransaction struct {
	commitCount   int
	rollbackCount int
	onRollback    func(context.Context)
}

func (t *fakeDBTransaction) Query(
	context.Context, string, ...any,
) (ports.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (t *fakeDBTransaction) QueryRow(
	context.Context, string, ...any,
) ports.DatabaseRow {
	return nil
}

func (t *fakeDBTransaction) Exec(
	context.Context, string, ...any,
) (ports.DatabaseResult, error) {
	return fakeDBResult(0), nil
}

func (t *fakeDBTransaction) Commit(context.Context) error {
	t.commitCount++
	return nil
}

func (t *fakeDBTransaction) Rollback(ctx context.Context) error {
	t.rollbackCount++
	if t.onRollback != nil {
		t.onRollback(ctx)
	}
	return nil
}

type fakeDBRows struct{}

func (fakeDBRows) Next() bool        { return false }
func (fakeDBRows) Scan(...any) error { return nil }
func (fakeDBRows) Close()            {}
func (fakeDBRows) Err() error        { return nil }

type fakeDBRow struct{}

func (fakeDBRow) Scan(...any) error { return nil }

type countingDBRows struct {
	closeCount int
}

func (r *countingDBRows) Next() bool        { return false }
func (r *countingDBRows) Scan(...any) error { return nil }
func (r *countingDBRows) Close()            { r.closeCount++ }
func (r *countingDBRows) Err() error        { return nil }

type countingDBRow struct {
	scanCount int
}

func (r *countingDBRow) Scan(...any) error {
	r.scanCount++
	return nil
}

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }
