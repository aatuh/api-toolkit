package idempotency_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

type exampleTokenAwareStore struct{}

func (exampleTokenAwareStore) Get(context.Context, string) (idempotency.Record, bool, error) {
	return idempotency.Record{}, false, nil
}

func (exampleTokenAwareStore) TryBegin(context.Context, string, idempotency.Record, time.Duration) (bool, error) {
	return true, nil
}

func (exampleTokenAwareStore) Save(context.Context, string, idempotency.Record, time.Duration) error {
	return nil
}

func (exampleTokenAwareStore) ReleaseReservation(_ context.Context, _ string, token string) error {
	if token == "" {
		return idempotency.ErrLegacyInFlightReservationMissingToken
	}
	return nil
}

func ExampleReservationReleaser() {
	var store idempotency.ReleasableStore = exampleTokenAwareStore{}

	record := idempotency.Record{
		State:            idempotency.StateInFlight,
		Status:           http.StatusAccepted,
		ReservationToken: "reservation-token",
	}
	ok, _ := store.TryBegin(context.Background(), "orders:create:1", record, 30*time.Second)
	_ = store.ReleaseReservation(context.Background(), "orders:create:1", record.ReservationToken)

	fmt.Println(ok)
	// Output: true
}
