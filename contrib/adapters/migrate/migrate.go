package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"time"

	// Register pgx stdlib driver for database/sql usage.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aatuh/api-toolkit/contrib/v2/migrator"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// Adapter implements runtime.Migrator using migrator.Runner.
type Adapter struct {
	log    ports.Logger
	db     *sql.DB
	runner *migrator.Runner
}

// Options configures migrator initialization.
type Options struct {
	DSN                string
	Dirs               []string // optional; multiple directories
	Table              string
	LockKey            int64
	AllowDangerousDown bool
	EmbeddedFSs        []fs.FS // optional; multiple embedded FS
	Log                ports.Logger
}

// New builds an Adapter and pings the database.
func New(opts Options) (ports.Migrator, error) {
	return NewWithContext(context.Background(), opts)
}

// NewWithContext builds an Adapter and pings the database with a context.
func NewWithContext(ctx context.Context, opts Options) (ports.Migrator, error) {
	if opts.Log == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if opts.DSN == "" {
		return nil, fmt.Errorf("dsn is required")
	}
	db, err := sql.Open("pgx", opts.DSN)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	r := migrator.New(db, migrator.Options{
		MigrationsDirs:     opts.Dirs,
		EmbeddedFSs:        opts.EmbeddedFSs,
		TableName:          opts.Table,
		LockKey:            opts.LockKey,
		AllowDangerousDown: opts.AllowDangerousDown,
		Logger: func(format string, args ...any) {
			opts.Log.Info(fmt.Sprintf(format, args...))
		},
	})
	return &Adapter{log: opts.Log, db: db, runner: r}, nil
}

// Close releases the underlying database handle.
func (a *Adapter) Close() error { return a.db.Close() }

// Up applies migrations.
func (a *Adapter) Up(ctx context.Context, _ string) error {
	// dir is ignored; Directory was configured in Options.
	return a.runner.Up(ctx)
}

// Down rolls back migrations.
func (a *Adapter) Down(ctx context.Context, _ string) error {
	return a.runner.Down(ctx)
}

// Status returns the current migration status.
func (a *Adapter) Status(ctx context.Context, _ string) (string, error) {
	return a.runner.Status(ctx)
}
