package bootstrap

import (
	"context"
	"io/fs"
	"time"

	"github.com/aatuh/api-toolkit/migrate"
	"github.com/aatuh/api-toolkit/ports"
)

// NewMigrator builds a migrator with either directories or embedded FS sources.
func NewMigrator(dsn, table string, lockKey int64, allowDown bool, log ports.Logger, dirs []string, embedded []fs.FS) (*migrate.Adapter, error) {
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
func RunUp(ctx context.Context, m *migrate.Adapter, dir string) error { return m.Up(ctx, dir) }

// RunDown runs migrations down with context and directory path.
func RunDown(ctx context.Context, m *migrate.Adapter, dir string) error { return m.Down(ctx, dir) }

// Status returns a text status of migrations.
func Status(ctx context.Context, m *migrate.Adapter, dir string) (string, error) {
	return m.Status(ctx, dir)
}

// WithTimeout derives a context with a default timeout for long-running migration ops.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		d = 15 * time.Minute
	}
	return context.WithTimeout(parent, d)
}
