package bootstrap

import (
	"context"
	"io/fs"
	"os"
	"time"

	"github.com/aatuh/api-toolkit/adapters/migrate"
	"github.com/aatuh/api-toolkit/config"
	"github.com/aatuh/api-toolkit/ports"
)

// NewMigrator builds a migrator with either directories or embedded FS sources.
func NewMigrator(dsn, table string, lockKey int64, allowDown bool, log ports.Logger, dirs []string, embedded []fs.FS) (ports.Migrator, error) {
	opts := migrate.Options{
		DSN:                dsn,
		Table:              table,
		LockKey:            lockKey,
		Log:                log,
		AllowDangerousDown: allowDown,
		Dirs:               dirs,
		EmbeddedFSs:        embedded,
	}
	return migrate.New(opts)
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

// RunMigrationsOrExit runs startup migrations using config defaults or exits on failure.
func RunMigrationsOrExit(ctx context.Context, cfg config.Config, log ports.Logger, embedded []fs.FS) {
	var dirs []string
	var embeddedFS []fs.FS
	if cfg.MigrationsDir == "-" || cfg.MigrationsDir == "" {
		embeddedFS = embedded
	} else {
		dirs = []string{cfg.MigrationsDir}
	}

	m, err := NewMigrator(
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
		os.Exit(1)
	}
	if err := RunUp(ctx, m, cfg.MigrationsDir); err != nil {
		log.Error("migrate up failed", "err", err)
		os.Exit(1)
	}
	_ = m.Close()
}
