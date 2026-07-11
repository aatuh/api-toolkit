package idempotency

import (
	"testing"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestPackageLocalStoreAliasesPreserveV3PortsIdentity(_ *testing.T) {
	var store Store
	requirePortsStore(store)

	var releaser ReservationReleaser
	requirePortsReservationReleaser(releaser)

	var releasable ReleasableStore
	requirePortsReleasableStore(releasable)
}

func requirePortsStore(ports.IdempotencyStore) {}

func requirePortsReservationReleaser(ports.IdempotencyReservationReleaser) {}

func requirePortsReleasableStore(ports.ReservationReleasableIdempotencyStore) {}
