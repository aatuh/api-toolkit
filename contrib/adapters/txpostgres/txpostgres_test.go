package txpostgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
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
func (p *fakeDBPool) Stat() ports.DatabaseStats  { return nil }

func (p *fakeDBPool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return p.conn, nil
}

type fakeDBConnection struct {
	tx           ports.DatabaseTransaction
	releaseCount int
}

func (c *fakeDBConnection) Query(
	context.Context, string, ...any,
) (ports.DatabaseRows, error) {
	return fakeDBRows{}, nil
}

func (c *fakeDBConnection) QueryRow(
	context.Context, string, ...any,
) ports.DatabaseRow {
	return nil
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

type fakeDBResult int64

func (r fakeDBResult) RowsAffected() int64 { return int64(r) }
