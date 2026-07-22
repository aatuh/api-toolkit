package idempotency

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aatuh/api-toolkit/v4/ports"
)

// ErrAmbiguousConfiguration reports conflicting grouped and legacy options.
var ErrAmbiguousConfiguration = errors.New("idempotency configuration is ambiguous")

// Limits configures request and replay response capture bounds. A zero field
// preserves the package default and does not override a legacy option.
type Limits struct {
	// MaxBodyBytes caps a buffered request body before hashing and replay.
	MaxBodyBytes int64
	// MaxResponseBytes caps a replayable response body before persistence.
	MaxResponseBytes int64
}

// Retention configures completed-record and in-flight reservation lifetimes. A
// zero field preserves the package default and does not override a legacy
// option.
type Retention struct {
	// CompletedTTL retains completed replay records.
	CompletedTTL time.Duration
	// InFlightTTL reserves a key while a request is in progress.
	InFlightTTL time.Duration
}

// FailurePolicy configures behavior when the idempotency store is unavailable.
type FailurePolicy struct {
	// FailOpen permits the request when the store cannot enforce idempotency.
	FailOpen bool
	// OnError receives internal store, reservation, and response-write failures.
	OnError func(error)
}

// Observability configures middleware logs and outcome notifications.
type Observability struct {
	// Logger receives middleware and compatibility telemetry.
	Logger ports.Logger
	// OnOutcome receives structured request outcomes.
	OnOutcome OutcomeHandler
}

// Compatibility configures temporary mixed-version in-flight recovery.
// ExposeRawLegacyKey is disabled by default because idempotency keys can be
// sensitive. LegacyAsync owns worker goroutines; callers must call
// Middleware.Close during graceful shutdown when it is enabled.
type Compatibility struct {
	// KnownInFlightTTLs records observed peer reservation lifetimes.
	KnownInFlightTTLs map[string]time.Duration
	// FailOnInFlightTTLMismatch rejects incompatible peer reservation lifetimes.
	FailOnInFlightTTLMismatch bool
	// FailOnClockSkewPreflight rejects a backwards-moving construction clock.
	FailOnClockSkewPreflight bool
	// ExposeRawLegacyKey permits raw idempotency keys in compatibility events.
	ExposeRawLegacyKey bool
	// LegacySink receives compatibility events.
	LegacySink LegacyInFlightCompatibilityEventSink
	// LegacyMetricSink receives bounded compatibility metric labels.
	LegacyMetricSink LegacyInFlightCompatibilityMetricSink
	// LegacyAsync dispatches compatibility events through owned workers.
	LegacyAsync bool
	// LegacySampleEvery emits every Nth compatibility event when greater than one.
	LegacySampleEvery int
	// OnLegacyInFlight receives compatibility events as a callback.
	OnLegacyInFlight LegacyInFlightCompatibilityHandler
}

func (opts Options) normalizeGroupedConfiguration() (Options, error) {
	if err := mergeInt64Option(&opts.MaxBodyBytes, opts.Limits.MaxBodyBytes, "max body bytes"); err != nil {
		return Options{}, err
	}
	if err := mergeInt64Option(&opts.MaxResponseBytes, opts.Limits.MaxResponseBytes, "max response bytes"); err != nil {
		return Options{}, err
	}
	if err := mergeDurationOption(&opts.TTL, opts.Retention.CompletedTTL, "completed ttl"); err != nil {
		return Options{}, err
	}
	if err := mergeDurationOption(&opts.InFlightTTL, opts.Retention.InFlightTTL, "in-flight ttl"); err != nil {
		return Options{}, err
	}
	if opts.Failure.FailOpen {
		opts.FailOpen = true
	}
	if opts.Failure.OnError != nil {
		if opts.OnError != nil {
			return Options{}, ambiguousConfiguration("on error")
		}
		opts.OnError = opts.Failure.OnError
	}
	if opts.Observability.Logger != nil {
		if opts.Logger != nil {
			return Options{}, ambiguousConfiguration("logger")
		}
		opts.Logger = opts.Observability.Logger
	}
	if opts.Observability.OnOutcome != nil {
		if opts.OnOutcome != nil {
			return Options{}, ambiguousConfiguration("on outcome")
		}
		opts.OnOutcome = opts.Observability.OnOutcome
	}
	if opts.Compatibility.KnownInFlightTTLs != nil {
		if opts.KnownInFlightTTLs != nil && !reflect.DeepEqual(opts.KnownInFlightTTLs, opts.Compatibility.KnownInFlightTTLs) {
			return Options{}, ambiguousConfiguration("known in-flight ttls")
		}
		opts.KnownInFlightTTLs = opts.Compatibility.KnownInFlightTTLs
	}
	if opts.Compatibility.FailOnInFlightTTLMismatch {
		opts.FailOnInFlightTTLMismatch = true
	}
	if opts.Compatibility.FailOnClockSkewPreflight {
		opts.FailOnInFlightClockSkewPreflight = true
	}
	if opts.Compatibility.ExposeRawLegacyKey {
		opts.LegacyInFlightCompatibilityRawKey = true
	}
	if opts.Compatibility.LegacySink != nil {
		if opts.LegacyInFlightCompatibilitySink != nil {
			return Options{}, ambiguousConfiguration("legacy compatibility sink")
		}
		opts.LegacyInFlightCompatibilitySink = opts.Compatibility.LegacySink
	}
	if opts.Compatibility.LegacyMetricSink != nil {
		if opts.LegacyInFlightCompatibilityMetricSink != nil {
			return Options{}, ambiguousConfiguration("legacy compatibility metric sink")
		}
		opts.LegacyInFlightCompatibilityMetricSink = opts.Compatibility.LegacyMetricSink
	}
	if opts.Compatibility.LegacyAsync {
		opts.LegacyInFlightCompatibilityAsync = true
	}
	if opts.Compatibility.LegacySampleEvery != 0 {
		if opts.LegacyInFlightCompatibilitySampleEvery != 0 && opts.LegacyInFlightCompatibilitySampleEvery != opts.Compatibility.LegacySampleEvery {
			return Options{}, ambiguousConfiguration("legacy compatibility sample rate")
		}
		opts.LegacyInFlightCompatibilitySampleEvery = opts.Compatibility.LegacySampleEvery
	}
	if opts.Compatibility.OnLegacyInFlight != nil {
		if opts.OnLegacyInFlightCompatibility != nil {
			return Options{}, ambiguousConfiguration("legacy compatibility callback")
		}
		opts.OnLegacyInFlightCompatibility = opts.Compatibility.OnLegacyInFlight
	}
	return opts, nil
}

func mergeInt64Option(legacy *int64, grouped int64, name string) error {
	if grouped == 0 {
		return nil
	}
	if *legacy != 0 && *legacy != grouped {
		return ambiguousConfiguration(name)
	}
	*legacy = grouped
	return nil
}

func mergeDurationOption(legacy *time.Duration, grouped time.Duration, name string) error {
	if grouped == 0 {
		return nil
	}
	if *legacy != 0 && *legacy != grouped {
		return ambiguousConfiguration(name)
	}
	*legacy = grouped
	return nil
}

func ambiguousConfiguration(name string) error {
	return fmt.Errorf("%w: %s is set in both grouped and legacy options", ErrAmbiguousConfiguration, name)
}
