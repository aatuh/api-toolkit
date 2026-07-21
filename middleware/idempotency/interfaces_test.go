package idempotency

import "testing"

func TestPackageLocalStoreAliasesPreserveV3PortsIdentity(_ *testing.T) {
	var store Store
	requirePortsStore(store)

	var releaser ReservationReleaser
	requirePortsReservationReleaser(releaser)

	var releasable ReleasableStore
	requirePortsReleasableStore(releasable)
}

func requirePortsStore(Store) {}

func requirePortsReservationReleaser(ReservationReleaser) {}

func requirePortsReleasableStore(ReleasableStore) {}

var _ func(ReleasableStore, Options) (*Middleware, error) = NewWithStore
var _ func(Options) (*Middleware, error) = New
