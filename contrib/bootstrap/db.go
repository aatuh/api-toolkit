package bootstrap

import (
	"context"
	"os"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// OpenAndPingDB opens a DB pool and verifies connectivity with a short timeout.
func OpenAndPingDB(ctx context.Context, dsn string, timeout time.Duration) (ports.DatabasePool, error) {
	pool, err := pgxpool.NewWithContext(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := pool.Ping(c); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// OpenPoolOrExit opens a DB pool and terminates the process if it fails.
func OpenPoolOrExit(ctx context.Context, dsn string, timeout time.Duration, log ports.Logger) ports.DatabasePool {
	if log == nil {
		log = ports.NopLogger{}
	}
	pool, err := OpenAndPingDB(ctx, dsn, timeout)
	if err != nil {
		log.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	return pool
}
