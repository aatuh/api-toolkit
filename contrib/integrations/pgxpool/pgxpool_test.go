package pgxpool

import (
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestNewRejectsInvalidDSN(t *testing.T) {
	if _, err := New("postgres://%zz"); err == nil {
		t.Fatal("expected invalid DSN error")
	}
}

func TestAdapterAliasesSatisfyPortInterfaces(t *testing.T) {
	var _ ports.DatabasePool = (*Adapter)(nil)
	var _ ports.DatabasePoolSnapshotProvider = (*Adapter)(nil)
}

func TestSnapshotAliasMatchesPortsSnapshot(t *testing.T) {
	snapshot := Snapshot{
		IdleConns:  2,
		TotalConns: 5,
	}

	portsSnapshot := acceptPortsSnapshot(snapshot)
	if portsSnapshot.IdleConns != 2 || portsSnapshot.TotalConns != 5 {
		t.Fatalf("snapshot alias drifted from ports snapshot: %+v", portsSnapshot)
	}
}

func acceptPortsSnapshot(snapshot ports.DatabasePoolSnapshot) ports.DatabasePoolSnapshot {
	return snapshot
}
