//go:build postgres

package testpostgres

import (
	"context"
	"errors"
	"testing"
)

func TestHarnessProvidesRealIsolatedPostgres(t *testing.T) {
	h := New(t)
	ctx := context.Background()
	if h.Version.Major != SupportedMajor {
		t.Fatalf("PostgreSQL major = %d, want %d", h.Version.Major, SupportedMajor)
	}
	if err := h.ApplyMigrations(ctx, Migration{
		Name: "widgets",
		SQL:  "CREATE TABLE widgets (id text PRIMARY KEY, created_at timestamptz NOT NULL)",
	}); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	tx, err := h.Begin(t, ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO widgets (id, created_at) VALUES ($1, $2)", h.NextText("widget"), h.FixedTime()); err != nil {
		t.Fatalf("insert fixture: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}
	var count int
	if err := h.Pool.QueryRow(ctx, "SELECT count(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("count fixtures: %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction cleanup count = %d, want 0", count)
	}

	if _, err := h.Pool.Exec(h.CanceledContext(t), "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query error = %v, want context cancellation", err)
	}

	conn, err := h.Pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()
	if err := h.TerminateConnection(ctx, conn); err != nil {
		t.Fatalf("TerminateConnection() error = %v", err)
	}
	if _, err := conn.Exec(ctx, "SELECT 1"); err == nil {
		t.Fatal("interrupted connection query unexpectedly succeeded")
	}
}

func TestHarnessParallelPostgresTestsDoNotShareSchema(t *testing.T) {
	for i := 0; i < 2; i++ {
		t.Run("isolated", func(t *testing.T) {
			t.Parallel()
			h := New(t)
			if err := h.ApplyMigrations(context.Background(), Migration{
				Name: "same-table-name",
				SQL:  "CREATE TABLE same_name (id integer PRIMARY KEY)",
			}); err != nil {
				t.Fatalf("ApplyMigrations() error = %v", err)
			}
		})
	}
}
