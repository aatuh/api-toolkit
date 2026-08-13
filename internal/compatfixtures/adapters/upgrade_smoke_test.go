//go:build downstreamcompat

// Package upgradesmoke is copied into a temporary downstream module by the
// compatibility corpus. It covers the public PostgreSQL and Redis adapters
// without requiring a service to be reachable during a compatibility check.
package upgradesmoke

import (
	"context"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/cacheredis"
	"github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
)

func TestAdapterConsumerUsesSafeConstructionAndValidation(t *testing.T) {
	pool, err := pgxpool.NewWithContext(context.Background(), "postgres://compat:compat@127.0.0.1:5432/compat")
	if err != nil {
		t.Fatalf("NewWithContext() error = %v", err)
	}
	pool.Close()

	store := cacheredis.New(nil, cacheredis.Options{KeyPrefix: "compat:", DefaultTTL: time.Minute})
	if _, _, err := store.Get(context.Background(), "invalid key with spaces"); err == nil {
		t.Fatal("Get() with an invalid cache key succeeded")
	}
}
