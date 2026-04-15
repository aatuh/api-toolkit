package ports

import (
	"testing"
	"time"
)

type stubDatabaseStats struct{}

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
