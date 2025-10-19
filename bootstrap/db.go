package bootstrap

import (
	"context"
	"time"

	"github.com/aatuh/api-toolkit/pgxpool"
	"github.com/aatuh/api-toolkit/ports"
)

// OpenAndPingDB opens a DB pool and verifies connectivity with a short timeout.
func OpenAndPingDB(ctx context.Context, dsn string, timeout time.Duration) (ports.DatabasePool, error) {
	pool, err := pgxpool.New(dsn)
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
