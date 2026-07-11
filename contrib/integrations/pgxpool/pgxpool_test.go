package pgxpool

import (
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
)

func TestNewRejectsInvalidDSN(t *testing.T) {
	if _, err := New("postgres://%zz"); err == nil {
		t.Fatal("expected invalid DSN error")
	}
}

func TestAdapterAliasesSatisfyPortInterfaces(t *testing.T) {
	var _ contracts.DatabasePool = (*Adapter)(nil)
	var _ contracts.DatabasePoolSnapshotProvider = (*Adapter)(nil)
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

func acceptPortsSnapshot(snapshot contracts.DatabasePoolSnapshot) contracts.DatabasePoolSnapshot {
	return snapshot
}
