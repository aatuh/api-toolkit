//go:build postgres

package txpostgres

import (
	"context"
	"errors"
	"testing"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestManagerCommitsAndRollsBackAgainstRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "transaction-fixture",
		SQL:  "CREATE TABLE transaction_fixture (id text PRIMARY KEY)",
	}); err != nil {
		t.Fatalf("create transaction fixture: %v", err)
	}
	manager := New(&pgxpooladapter.Adapter{Pool: h.Pool})
	if err := manager.WithinTx(ctx, func(txCtx context.Context) error {
		_, err := FromCtx(txCtx, nil).Exec(txCtx, "INSERT INTO transaction_fixture (id) VALUES ($1)", "committed")
		return err
	}); err != nil {
		t.Fatalf("WithinTx() commit error = %v", err)
	}
	rollback := errors.New("rollback fixture")
	if err := manager.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := FromCtx(txCtx, nil).Exec(txCtx, "INSERT INTO transaction_fixture (id) VALUES ($1)", "rolled-back"); err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatalf("WithinTx() rollback error = %v", err)
	}
	var count int
	if err := h.Pool.QueryRow(ctx, "SELECT count(*) FROM transaction_fixture").Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed row count = (%d, %v), want 1", count, err)
	}
	if err := manager.WithinTx(h.CanceledContext(t), func(context.Context) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("WithinTx() canceled context error = %v", err)
	}
}
