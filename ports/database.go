package ports

import (
	"context"
	"time"
)

// DatabasePool defines the interface for database connection pooling.
type DatabasePool interface {
	Ping(ctx context.Context) error
	Close()
	Acquire(ctx context.Context) (DatabaseConnection, error)
}

// DatabasePoolSnapshotProvider is an optional capability for pools that can
// expose plain-value pool statistics.
type DatabasePoolSnapshotProvider interface {
	StatSnapshot() DatabasePoolSnapshot
}

// DatabaseConnection defines the interface for individual database connections.
type DatabaseConnection interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Begin(ctx context.Context) (DatabaseTransaction, error)
	Release()
}

// DatabaseTransaction defines the interface for database transactions.
type DatabaseTransaction interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DatabaseRows defines the interface for query result rows.
type DatabaseRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// DatabaseRow defines the interface for a single query result row.
type DatabaseRow interface {
	Scan(dest ...any) error
}

// DatabaseResult defines the interface for query execution results.
type DatabaseResult interface {
	RowsAffected() int64
}

// DatabasePoolSnapshot captures database pool stats as plain values.
type DatabasePoolSnapshot struct {
	AcquireCount         int64
	AcquireDuration      time.Duration
	AcquiredConns        int32
	CanceledAcquireCount int64
	ConstructingConns    int32
	EmptyAcquireCount    int64
	IdleConns            int32
	MaxConns             int32
	NewConnsCount        int64
	TotalConns           int32
}

// SnapshotDatabasePoolStats copies pool stats into a value snapshot.
func SnapshotDatabasePoolStats(pool DatabasePool) DatabasePoolSnapshot {
	if pool == nil {
		return DatabasePoolSnapshot{}
	}
	if snapshotter, ok := pool.(DatabasePoolSnapshotProvider); ok {
		return snapshotter.StatSnapshot()
	}
	return DatabasePoolSnapshot{}
}
