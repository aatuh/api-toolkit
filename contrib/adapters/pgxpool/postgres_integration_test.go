//go:build postgres

package pgxpool

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestAdapterUsesRealPostgresAndSurfacesCancellationAndConnectionLoss(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	pool, err := NewWithContext(ctx, h.DatabaseURL())
	if err != nil {
		t.Fatalf("NewWithContext() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer conn.Release()
	var value int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil || value != 1 {
		t.Fatalf("QueryRow() = (%d, %v)", value, err)
	}
	if _, err := pool.Acquire(h.CanceledContext(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire() canceled context error = %v", err)
	}
	underlying, ok := conn.(*Connection)
	if !ok {
		t.Fatalf("connection type = %T, want *Connection", conn)
	}
	if err := h.TerminateConnection(ctx, underlying.Conn); err != nil {
		t.Fatalf("TerminateConnection() error = %v", err)
	}
	if _, err := underlying.Exec(ctx, "SELECT 1"); err == nil {
		t.Fatal("query on interrupted PostgreSQL connection unexpectedly succeeded")
	}
}
