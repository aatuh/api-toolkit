package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/aatuh/api-toolkit/migrator"
	"github.com/aatuh/api-toolkit/ports"
)

// Adapter implements runtime.Migrator using migrator.Runner.
type Adapter struct {
	log    ports.Logger
	db     *sql.DB
	runner *migrator.Runner
}

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
func New(opts Options) (*Adapter, error) {
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
	if err := db.Ping(); err != nil {
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

func (a *Adapter) Close() error { return a.db.Close() }

func (a *Adapter) Up(ctx context.Context, dir string) error {
	// dir is ignored; Directory was configured in Options.
	return a.runner.Up(ctx)
}

func (a *Adapter) Down(ctx context.Context, dir string) error {
	return a.runner.Down(ctx)
}

func (a *Adapter) Status(ctx context.Context, dir string) (string, error) {
	return a.runner.Status(ctx)
}
