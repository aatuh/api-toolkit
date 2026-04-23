package ports

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubDatabaseStats struct{}

var errAcquireNotImplemented = errors.New("acquire not implemented")

func (stubDatabaseStats) AcquireCount() int64            { return 11 }
func (stubDatabaseStats) AcquireDuration() time.Duration { return 12 * time.Millisecond }
func (stubDatabaseStats) AcquiredConns() int32           { return 13 }
func (stubDatabaseStats) CanceledAcquireCount() int64    { return 14 }
func (stubDatabaseStats) ConstructingConns() int32       { return 15 }
func (stubDatabaseStats) EmptyAcquireCount() int64       { return 16 }
func (stubDatabaseStats) IdleConns() int32               { return 17 }
func (stubDatabaseStats) MaxConns() int32                { return 18 }
func (stubDatabaseStats) NewConnsCount() int64           { return 19 }
func (stubDatabaseStats) TotalConns() int32              { return 20 }

type snapshotOnlyDatabasePool struct {
	snapshot  DatabasePoolSnapshot
	statCalls int
}

func (*snapshotOnlyDatabasePool) Ping(context.Context) error { return nil }

func (*snapshotOnlyDatabasePool) Close() {}

func (*snapshotOnlyDatabasePool) Acquire(context.Context) (DatabaseConnection, error) {
	return nil, errAcquireNotImplemented
}

func (p *snapshotOnlyDatabasePool) Stat() DatabaseStats {
	p.statCalls++
	return stubDatabaseStats{}
}

func (p *snapshotOnlyDatabasePool) StatSnapshot() DatabasePoolSnapshot {
	return p.snapshot
}

type legacyDatabasePool struct{}

func (*legacyDatabasePool) Ping(context.Context) error { return nil }

func (*legacyDatabasePool) Close() {}

func (*legacyDatabasePool) Acquire(context.Context) (DatabaseConnection, error) {
	return nil, errAcquireNotImplemented
}

func (*legacyDatabasePool) Stat() DatabaseStats {
	return stubDatabaseStats{}
}

func TestSnapshotDatabaseStats(t *testing.T) {
	got := SnapshotDatabaseStats(stubDatabaseStats{})

	if got.AcquireCount != 11 {
		t.Fatalf("AcquireCount = %d", got.AcquireCount)
	}
	if got.AcquireDuration != 12*time.Millisecond {
		t.Fatalf("AcquireDuration = %v", got.AcquireDuration)
	}
	if got.AcquiredConns != 13 {
		t.Fatalf("AcquiredConns = %d", got.AcquiredConns)
	}
	if got.CanceledAcquireCount != 14 {
		t.Fatalf("CanceledAcquireCount = %d", got.CanceledAcquireCount)
	}
	if got.ConstructingConns != 15 {
		t.Fatalf("ConstructingConns = %d", got.ConstructingConns)
	}
	if got.EmptyAcquireCount != 16 {
		t.Fatalf("EmptyAcquireCount = %d", got.EmptyAcquireCount)
	}
	if got.IdleConns != 17 {
		t.Fatalf("IdleConns = %d", got.IdleConns)
	}
	if got.MaxConns != 18 {
		t.Fatalf("MaxConns = %d", got.MaxConns)
	}
	if got.NewConnsCount != 19 {
		t.Fatalf("NewConnsCount = %d", got.NewConnsCount)
	}
	if got.TotalConns != 20 {
		t.Fatalf("TotalConns = %d", got.TotalConns)
	}
}

func TestSnapshotDatabaseStatsNil(t *testing.T) {
	got := SnapshotDatabaseStats(nil)
	if got != (DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}

func TestSnapshotDatabasePoolStatsUsesSnapshotProvider(t *testing.T) {
	pool := &snapshotOnlyDatabasePool{
		snapshot: DatabasePoolSnapshot{
			AcquireCount: 41,
			IdleConns:    42,
			TotalConns:   43,
		},
	}

	got := SnapshotDatabasePoolStats(pool)

	if got != pool.snapshot {
		t.Fatalf("expected snapshot %+v, got %+v", pool.snapshot, got)
	}
	if pool.statCalls != 0 {
		t.Fatalf("expected no legacy Stat calls, got %d", pool.statCalls)
	}
}

func TestSnapshotDatabasePoolStatsFallsBackToLegacyStats(t *testing.T) {
	got := SnapshotDatabasePoolStats(&legacyDatabasePool{})

	if got.AcquireCount != 11 {
		t.Fatalf("AcquireCount = %d", got.AcquireCount)
	}
	if got.TotalConns != 20 {
		t.Fatalf("TotalConns = %d", got.TotalConns)
	}
}

func TestSnapshotDatabasePoolStatsNil(t *testing.T) {
	got := SnapshotDatabasePoolStats(nil)
	if got != (DatabasePoolSnapshot{}) {
		t.Fatalf("expected zero snapshot, got %+v", got)
	}
}
