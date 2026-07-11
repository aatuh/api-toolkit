package bootstrap

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/config"
	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestRunMigrationsReturnsInitError(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DatabaseURL:   "postgres://db.example/internal",
		MigrationsDir: "-",
	}
	wantErr := errors.New("dial failed")

	err := runMigrations(context.Background(), cfg, nil, []fs.FS{fstest.MapFS{}}, func(context.Context, string, string, int64, bool, ports.Logger, []string, []fs.FS) (contracts.Migrator, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runMigrations() error = %v, want %v", err, wantErr)
	}
}

func TestRunMigrationsUsesDirectorySourceAndClosesMigrator(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DatabaseURL:   "postgres://db.example/internal",
		MigrationsDir: "db/migrations",
	}
	migrator := &stubMigrator{}

	err := runMigrations(context.Background(), cfg, nil, nil, func(_ context.Context, dsn, table string, lockKey int64, allowDown bool, _ ports.Logger, dirs []string, embedded []fs.FS) (contracts.Migrator, error) {
		if dsn != cfg.DatabaseURL {
			t.Fatalf("dsn = %q, want %q", dsn, cfg.DatabaseURL)
		}
		if table != "schema_migrations" {
			t.Fatalf("table = %q, want schema_migrations", table)
		}
		if lockKey != 0 {
			t.Fatalf("lockKey = %d, want 0", lockKey)
		}
		if allowDown {
			t.Fatal("allowDown = true, want false")
		}
		if len(dirs) != 1 || dirs[0] != cfg.MigrationsDir {
			t.Fatalf("dirs = %#v, want [%q]", dirs, cfg.MigrationsDir)
		}
		if len(embedded) != 0 {
			t.Fatalf("embedded = %#v, want nil", embedded)
		}
		return migrator, nil
	})
	if err != nil {
		t.Fatalf("runMigrations() error = %v", err)
	}
	if migrator.upDir != cfg.MigrationsDir {
		t.Fatalf("Up() dir = %q, want %q", migrator.upDir, cfg.MigrationsDir)
	}
	if !migrator.closed {
		t.Fatal("Close() not called")
	}
}

func TestRunMigrationsUsesEmbeddedSourcesWhenRequested(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DatabaseURL:   "postgres://db.example/internal",
		MigrationsDir: "-",
	}
	embedded := []fs.FS{fstest.MapFS{"001_init.sql": {Data: []byte("select 1;")}}}
	migrator := &stubMigrator{}

	err := runMigrations(context.Background(), cfg, nil, embedded, func(_ context.Context, _ string, _ string, _ int64, _ bool, _ ports.Logger, dirs []string, sources []fs.FS) (contracts.Migrator, error) {
		if len(dirs) != 0 {
			t.Fatalf("dirs = %#v, want nil", dirs)
		}
		if len(sources) != len(embedded) {
			t.Fatalf("embedded sources = %d, want %d", len(sources), len(embedded))
		}
		return migrator, nil
	})
	if err != nil {
		t.Fatalf("runMigrations() error = %v", err)
	}
	if migrator.upDir != cfg.MigrationsDir {
		t.Fatalf("Up() dir = %q, want %q", migrator.upDir, cfg.MigrationsDir)
	}
}

func TestRunMigrationsReturnsCloseError(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		DatabaseURL:   "postgres://db.example/internal",
		MigrationsDir: "db/migrations",
	}
	wantErr := errors.New("close failed")

	err := runMigrations(context.Background(), cfg, nil, nil, func(context.Context, string, string, int64, bool, ports.Logger, []string, []fs.FS) (contracts.Migrator, error) {
		return &stubMigrator{closeErr: wantErr}, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runMigrations() error = %v, want %v", err, wantErr)
	}
}

func TestNewMigratorUsesBoundedStartupContext(t *testing.T) {
	wantErr := errors.New("stop after context capture")
	cfgDirs := []string{"db/migrations"}
	cfgEmbedded := []fs.FS{fstest.MapFS{}}

	_, err := newMigratorWithStartupTimeout("postgres://db.example/internal", "schema_migrations", 7, true, ports.NopLogger{}, cfgDirs, cfgEmbedded, func(ctx context.Context, dsn, table string, lockKey int64, allowDown bool, log ports.Logger, dirs []string, embedded []fs.FS) (contracts.Migrator, error) {
		assertDeadlineWithin(t, ctx, defaultStartupTimeout)
		if dsn != "postgres://db.example/internal" {
			t.Fatalf("dsn = %q", dsn)
		}
		if table != "schema_migrations" {
			t.Fatalf("table = %q", table)
		}
		if lockKey != 7 {
			t.Fatalf("lockKey = %d", lockKey)
		}
		if !allowDown {
			t.Fatal("allowDown = false, want true")
		}
		if len(dirs) != len(cfgDirs) || dirs[0] != cfgDirs[0] {
			t.Fatalf("dirs = %#v, want %#v", dirs, cfgDirs)
		}
		if len(embedded) != len(cfgEmbedded) {
			t.Fatalf("embedded = %#v, want %#v", embedded, cfgEmbedded)
		}
		if log == nil {
			t.Fatal("log = nil, want logger")
		}
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("newMigratorWithStartupTimeout() error = %v, want %v", err, wantErr)
	}
}

type stubMigrator struct {
	upDir    string
	closed   bool
	closeErr error
}

func (m *stubMigrator) Up(_ context.Context, dir string) error {
	m.upDir = dir
	return nil
}

func (m *stubMigrator) Down(context.Context, string) error { return nil }

func (m *stubMigrator) Status(context.Context, string) (string, error) { return "", nil }

func (m *stubMigrator) Close() error {
	m.closed = true
	return m.closeErr
}

func assertDeadlineWithin(t *testing.T, ctx context.Context, want time.Duration) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context missing deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatalf("context deadline already expired: %s", remaining)
	}
	if remaining > want || remaining < want-(time.Second) {
		t.Fatalf("context deadline = %s from now, want roughly %s", remaining, want)
	}
}
