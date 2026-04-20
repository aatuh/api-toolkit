package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/migrate"
	"github.com/aatuh/api-toolkit/contrib/v2/config"
	"github.com/aatuh/api-toolkit/v2/ports"
)

type migratorFactory func(context.Context, string, string, int64, bool, ports.Logger, []string, []fs.FS) (ports.Migrator, error)

// NewMigrator builds a migrator with either directories or embedded FS sources.
func NewMigrator(dsn, table string, lockKey int64, allowDown bool, log ports.Logger, dirs []string, embedded []fs.FS) (ports.Migrator, error) {
	return NewMigratorWithContext(context.Background(), dsn, table, lockKey, allowDown, log, dirs, embedded)
}

// NewMigratorWithContext builds a migrator with either directories or embedded FS sources.
func NewMigratorWithContext(ctx context.Context, dsn, table string, lockKey int64, allowDown bool, log ports.Logger, dirs []string, embedded []fs.FS) (ports.Migrator, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	opts := migrate.Options{
		DSN:                dsn,
		Table:              table,
		LockKey:            lockKey,
		Log:                log,
		AllowDangerousDown: allowDown,
		Dirs:               dirs,
		EmbeddedFSs:        embedded,
	}
	return migrate.NewWithContext(ctx, opts)
}

// RunUp runs migrations up with context and directory path.
func RunUp(ctx context.Context, m ports.Migrator, dir string) error { return m.Up(ctx, dir) }

// RunDown runs migrations down with context and directory path.
func RunDown(ctx context.Context, m ports.Migrator, dir string) error { return m.Down(ctx, dir) }

// Status returns a text status of migrations.
func Status(ctx context.Context, m ports.Migrator, dir string) (string, error) {
	return m.Status(ctx, dir)
}

// WithTimeout derives a context with a default timeout for long-running migration ops.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		d = 15 * time.Minute
	}
	return context.WithTimeout(parent, d)
}

// RunMigrations runs startup migrations using config defaults and returns errors
// to the caller. Reusable library code should prefer this function.
func RunMigrations(ctx context.Context, cfg config.Config, log ports.Logger, embedded []fs.FS) (err error) {
	return runMigrations(ctx, cfg, log, embedded, NewMigratorWithContext)
}

func runMigrations(ctx context.Context, cfg config.Config, log ports.Logger, embedded []fs.FS, newMigrator migratorFactory) (err error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	dirs, embeddedFS := migrationSources(cfg.MigrationsDir, embedded)

	m, err := newMigrator(
		ctx,
		cfg.DatabaseURL,
		"schema_migrations",
		0,
		false,
		log,
		dirs,
		embeddedFS,
	)
	if err != nil {
		log.Error("migrator init failed", "err", err)
		return fmt.Errorf("bootstrap: init migrator: %w", err)
	}
	defer func() {
		closeErr := m.Close()
		if closeErr == nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("bootstrap: close migrator: %w", closeErr)
			return
		}
		log.Error("migrator close failed", "err", closeErr)
	}()

	if err := RunUp(ctx, m, cfg.MigrationsDir); err != nil {
		log.Error("migrate up failed", "err", err)
		return fmt.Errorf("bootstrap: migrate up: %w", err)
	}
	return nil
}

func migrationSources(dir string, embedded []fs.FS) ([]string, []fs.FS) {
	if dir == "-" || dir == "" {
		return nil, embedded
	}
	return []string{dir}, nil
}

// RunMigrationsOrExit runs startup migrations using config defaults or exits on
// failure. This helper is intended for binaries; reusable library code should
// prefer RunMigrations.
func RunMigrationsOrExit(ctx context.Context, cfg config.Config, log ports.Logger, embedded []fs.FS) {
	if err := RunMigrations(ctx, cfg, log, embedded); err != nil {
		if log == nil {
			log = ports.NopLogger{}
		}
		log.Error("startup migrations failed", "err", err)
		os.Exit(1)
	}
}
