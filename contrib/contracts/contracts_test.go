package contracts

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errAcquireNotImplemented = errors.New("acquire not implemented")

type snapshotOnlyDatabasePool struct {
	snapshot DatabasePoolSnapshot
}

func (*snapshotOnlyDatabasePool) Ping(context.Context) error { return nil }

func (*snapshotOnlyDatabasePool) Close() {}

func (*snapshotOnlyDatabasePool) Acquire(context.Context) (DatabaseConnection, error) {
	return nil, errAcquireNotImplemented
}

func (p *snapshotOnlyDatabasePool) StatSnapshot() DatabasePoolSnapshot {
	return p.snapshot
}

type databasePoolWithoutStats struct{}

func (*databasePoolWithoutStats) Ping(context.Context) error { return nil }

func (*databasePoolWithoutStats) Close() {}

func (*databasePoolWithoutStats) Acquire(context.Context) (DatabaseConnection, error) {
	return nil, errAcquireNotImplemented
}

func TestSnapshotDatabasePoolStatsUsesSnapshotProvider(t *testing.T) {
	pool := &snapshotOnlyDatabasePool{
		snapshot: DatabasePoolSnapshot{
			AcquireCount:    41,
			AcquireDuration: 12 * time.Millisecond,
			IdleConns:       42,
			TotalConns:      43,
		},
	}

	got := SnapshotDatabasePoolStats(pool)
	if got != pool.snapshot {
		t.Fatalf("expected snapshot %+v, got %+v", pool.snapshot, got)
	}
}

func TestSnapshotDatabasePoolStatsWithoutProviderReturnsZeroSnapshot(t *testing.T) {
	got := SnapshotDatabasePoolStats(&databasePoolWithoutStats{})
	if got != (DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}

func TestSnapshotDatabasePoolStatsNil(t *testing.T) {
	got := SnapshotDatabasePoolStats(nil)
	if got != (DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}
