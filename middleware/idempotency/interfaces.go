package idempotency

import "github.com/aatuh/api-toolkit/v3/ports"

// Store is the package-local idempotency persistence contract.
//
// It is an alias for ports.IdempotencyStore during v3 compatibility. New store
// implementations should import this package instead of adding to root ports.
type Store = ports.IdempotencyStore

// ReservationReleaser is the token-aware reservation cleanup contract.
//
// It is an alias for ports.IdempotencyReservationReleaser during v3
// compatibility.
type ReservationReleaser = ports.IdempotencyReservationReleaser

// ReleasableStore combines idempotency persistence and token-aware release.
//
// It is an alias for ports.ReservationReleasableIdempotencyStore during v3
// compatibility.
type ReleasableStore = ports.ReservationReleasableIdempotencyStore
