package ports_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v3/ports"
)

type exampleTokenAwareStore struct{}

func (exampleTokenAwareStore) Get(context.Context, string) (ports.IdempotencyRecord, bool, error) {
	return ports.IdempotencyRecord{}, false, nil
}

func (exampleTokenAwareStore) TryBegin(context.Context, string, ports.IdempotencyRecord, time.Duration) (bool, error) {
	return true, nil
}

func (exampleTokenAwareStore) Save(context.Context, string, ports.IdempotencyRecord, time.Duration) error {
	return nil
}

func (exampleTokenAwareStore) Release(context.Context, string) error {
	return nil
}

func (exampleTokenAwareStore) ReleaseReservation(_ context.Context, _ string, token string) error {
	if token == "" {
		return ports.ErrLegacyInFlightReservationMissingToken
	}
	return nil
}

func ExampleIdempotencyReservationReleaser() {
	var store ports.ReservationReleasableIdempotencyStore = exampleTokenAwareStore{}

	record := ports.IdempotencyRecord{
		State:            ports.IdempotencyStateInFlight,
		Status:           http.StatusAccepted,
		ReservationToken: "reservation-token",
	}
	ok, _ := store.TryBegin(context.Background(), "orders:create:1", record, 30*time.Second)
	_ = store.ReleaseReservation(context.Background(), "orders:create:1", record.ReservationToken)

	fmt.Println(ok)
	// Output: true
}
