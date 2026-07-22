//go:build postgres

package txpostgres

import (
	"context"
	"testing"

	adapterpgxpool "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestIntegrationManagerUsesRealPostgresTransactions(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{Name: "integration-transaction", SQL: "CREATE TABLE integration_transaction_fixture (id text PRIMARY KEY)"}); err != nil {
		t.Fatalf("create transaction fixture: %v", err)
	}
	manager := New(&adapterpgxpool.Adapter{Pool: h.Pool})
	if err := manager.WithinTx(ctx, func(txCtx context.Context) error {
		_, err := FromCtx(txCtx, nil).Exec(txCtx, "INSERT INTO integration_transaction_fixture (id) VALUES ($1)", "committed")
		return err
	}); err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	var count int
	if err := h.Pool.QueryRow(ctx, "SELECT count(*) FROM integration_transaction_fixture").Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed row count = (%d, %v)", count, err)
	}
}
