//go:build postgres

package migrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
	"github.com/aatuh/api-toolkit/v4/ports"
)

func TestAdapterRunsRealPostgresMigrations(t *testing.T) {
	h := testpostgres.New(t)
	dir := t.TempDir()
	write := func(name, sql string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sql), 0o600); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}
	write("20260722000000_create_adapter_fixture.up.sql", "CREATE TABLE adapter_migration_fixture (id text PRIMARY KEY);")
	write("20260722000000_create_adapter_fixture.down.sql", "DROP TABLE adapter_migration_fixture;")
	adapter, err := NewWithContext(context.Background(), Options{
		DSN:                h.DatabaseURL(),
		Dirs:               []string{dir},
		Table:              "adapter_integration_migrations",
		AllowDangerousDown: true,
		Log:                ports.NopLogger{},
	})
	if err != nil {
		t.Fatalf("NewWithContext() error = %v", err)
	}
	t.Cleanup(func() { _ = adapter.Close() })
	ctx := context.Background()
	if err := adapter.Up(ctx, ""); err != nil {
		t.Fatalf("Up() error = %v", err)
	}
	status, err := adapter.Status(ctx, "")
	if err != nil || !strings.Contains(status, "20260722000000") {
		t.Fatalf("Status() = (%q, %v)", status, err)
	}
	if _, err := h.Pool.Exec(ctx, "INSERT INTO public.adapter_migration_fixture (id) VALUES ($1)", "fixture"); err != nil {
		t.Fatalf("query migrated table: %v", err)
	}
	if err := adapter.Down(ctx, ""); err != nil {
		t.Fatalf("Down() error = %v", err)
	}
}
