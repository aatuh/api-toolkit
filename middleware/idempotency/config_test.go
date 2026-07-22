package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewWithStoreAppliesGroupedConfiguration(t *testing.T) {
	store := newMemoryStore()
	onError := func(error) {}
	onOutcome := func(context.Context, OutcomeEvent) {}
	mw, err := NewWithStore(store, Options{
		Limits: Limits{
			MaxBodyBytes:     128,
			MaxResponseBytes: 256,
		},
		Retention: Retention{
			CompletedTTL: time.Hour,
			InFlightTTL:  2 * time.Minute,
		},
		Failure: FailurePolicy{
			FailOpen: true,
			OnError:  onError,
		},
		Observability: Observability{
			OnOutcome: onOutcome,
		},
		Compatibility: Compatibility{
			KnownInFlightTTLs:         map[string]time.Duration{"peer": 2 * time.Minute},
			FailOnInFlightTTLMismatch: true,
			ExposeRawLegacyKey:        true,
			LegacySampleEvery:         10,
		},
	})
	if err != nil {
		t.Fatalf("NewWithStore() error = %v", err)
	}
	defer mw.Close()

	if got := mw.opts.MaxBodyBytes; got != 128 {
		t.Fatalf("MaxBodyBytes = %d, want 128", got)
	}
	if got := mw.opts.MaxResponseBytes; got != 256 {
		t.Fatalf("MaxResponseBytes = %d, want 256", got)
	}
	if got := mw.opts.TTL; got != time.Hour {
		t.Fatalf("TTL = %s, want %s", got, time.Hour)
	}
	if got := mw.opts.InFlightTTL; got != 2*time.Minute {
		t.Fatalf("InFlightTTL = %s, want %s", got, 2*time.Minute)
	}
	if !mw.opts.FailOpen || mw.opts.OnError == nil || mw.opts.OnOutcome == nil {
		t.Fatalf("grouped failure/observability options were not applied: %#v", mw.opts)
	}
	if !mw.opts.LegacyInFlightCompatibilityRawKey || mw.opts.LegacyInFlightCompatibilitySampleEvery != 10 {
		t.Fatalf("grouped compatibility options were not applied: %#v", mw.opts)
	}
}

func TestNewWithStoreRejectsConflictingGroupedAndLegacyConfiguration(t *testing.T) {
	store := newMemoryStore()
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "limits",
			opts: Options{MaxBodyBytes: 128, Limits: Limits{MaxBodyBytes: 256}},
		},
		{
			name: "retention",
			opts: Options{TTL: time.Hour, Retention: Retention{CompletedTTL: 2 * time.Hour}},
		},
		{
			name: "failure callback",
			opts: Options{OnError: func(error) {}, Failure: FailurePolicy{OnError: func(error) {}}},
		},
		{
			name: "observability callback",
			opts: Options{OnOutcome: func(context.Context, OutcomeEvent) {}, Observability: Observability{OnOutcome: func(context.Context, OutcomeEvent) {}}},
		},
		{
			name: "compatibility sample rate",
			opts: Options{LegacyInFlightCompatibilitySampleEvery: 2, Compatibility: Compatibility{LegacySampleEvery: 3}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewWithStore(store, tc.opts)
			if !errors.Is(err, ErrAmbiguousConfiguration) {
				t.Fatalf("NewWithStore() error = %v, want ErrAmbiguousConfiguration", err)
			}
		})
	}
}

func TestLegacyCompatibilityAsyncSinkCloseWaitsForWorkersAndRejectsNewEvents(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	sink := newLegacyInFlightCompatibilityAsyncSink(LegacyInFlightCompatibilitySinkFunc(func(context.Context, LegacyInFlightCompatibilityEvent) {
		close(started)
		<-release
	}), &captureLogger{})

	sink.Emit(context.Background(), LegacyInFlightCompatibilityEvent{})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("async worker did not start")
	}

	closed := make(chan struct{})
	go func() {
		sink.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned before in-flight telemetry completed")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not stop async workers")
	}

	sink.Emit(context.Background(), LegacyInFlightCompatibilityEvent{})
}
