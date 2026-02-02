package migrator

import (
	"errors"
	"testing"
)

func TestPendingUpChecksumMismatch(t *testing.T) {
	r := &Runner{
		migrations: []*Migration{
			{
				Version:  20240101000000,
				Name:     "init",
				Dir:      "up",
				Checksum: "expected",
			},
		},
	}
	applied := []appliedRow{
		{
			Version:  20240101000000,
			Name:     "init",
			Checksum: "actual",
			Success:  true,
		},
	}
	_, err := r.pendingUp(applied)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	var mismatch *ChecksumMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected ChecksumMismatchError, got %T", err)
	}
	if mismatch.Version != 20240101000000 || mismatch.Name != "init" {
		t.Fatalf("unexpected mismatch details: %#v", mismatch)
	}
}
