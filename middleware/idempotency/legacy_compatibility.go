package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

const legacyInFlightCompatibilityUnknownKeyValue = "[redacted]"
const legacyInFlightCompatibilityAsyncQueueSize = 1024
const legacyInFlightCompatibilityAsyncWorkers = 4

var (
	ErrLegacyInFlightTTLMismatch            = errors.New("idempotency in-flight ttl mismatch in rollout contract")
	ErrLegacyInFlightClockSkewPreflightRisk = errors.New("idempotency legacy in-flight clock preflight risk")
)

// KeyFunc extracts an idempotency key from the request.
type KeyFunc func(*http.Request) string

// HashFunc computes a request hash used to detect key reuse with different payloads.
type HashFunc func(*http.Request, []byte) (string, error)

// LegacyInFlightCompatibilityEventName identifies structured compatibility events for
// mixed-version in-flight migrations.
type LegacyInFlightCompatibilityEventName string

const (
	// LegacyInFlightCompatibilityEntered reports that legacy fallback was considered.
	LegacyInFlightCompatibilityEntered LegacyInFlightCompatibilityEventName = "legacy_in_flight_fallback_entered"
	// LegacyInFlightCompatibilityRecovered reports legacy fallback was successful.
	LegacyInFlightCompatibilityRecovered LegacyInFlightCompatibilityEventName = "legacy_in_flight_fallback_recovered"
	// LegacyInFlightCompatibilityRejected reports fallback was explicitly rejected.
	LegacyInFlightCompatibilityRejected LegacyInFlightCompatibilityEventName = "legacy_in_flight_fallback_rejected"
	// LegacyInFlightCompatibilityUnknown reports fallback ended with an unexpected error.
	LegacyInFlightCompatibilityUnknown LegacyInFlightCompatibilityEventName = "legacy_in_flight_fallback_unknown"
)

// LegacyInFlightCompatibilityEvent contains structured fields for idempotency
// compatibility telemetry.
type LegacyInFlightCompatibilityEvent struct {
	Method    string
	Path      string
	Key       string
	StoreType string
	Outcome   LegacyInFlightCompatibilityEventName
	Error     string
}

const (
	legacyInFlightCompatibilityEventMethodLabel    = "method"
	legacyInFlightCompatibilityEventPathLabel      = "path"
	legacyInFlightCompatibilityEventStoreTypeLabel = "store_type"
	legacyInFlightCompatibilityEventOutcomeLabel   = "outcome"
	legacyInFlightCompatibilityEventKeyLabel       = "key"
	legacyInFlightCompatibilityEventErrorLabel     = "error"
)

// MetricLabels returns the canonical metric label set for compatibility telemetry.
// Keep these names stable for dashboard migrations.
func (event LegacyInFlightCompatibilityEvent) MetricLabels() map[string]string {
	return map[string]string{
		legacyInFlightCompatibilityEventMethodLabel:    event.Method,
		legacyInFlightCompatibilityEventPathLabel:      event.Path,
		legacyInFlightCompatibilityEventStoreTypeLabel: event.StoreType,
		legacyInFlightCompatibilityEventOutcomeLabel:   string(event.Outcome),
		legacyInFlightCompatibilityEventKeyLabel:       event.Key,
		legacyInFlightCompatibilityEventErrorLabel:     event.Error,
	}
}

// LegacyInFlightCompatibilityHandler receives telemetry from legacy recovery paths.
type LegacyInFlightCompatibilityHandler func(context.Context, LegacyInFlightCompatibilityEvent)

// LegacyInFlightCompatibilityMetricLabels is the canonical compatibility label set for
// metric adapters.
type LegacyInFlightCompatibilityMetricLabels map[string]string

// LegacyInFlightCompatibilityMetricSink emits structured compatibility metrics.
//
// Implementations should avoid blocking and avoid panicking, since compatibility
// telemetry must not alter request behavior.
type LegacyInFlightCompatibilityMetricSink interface {
	Emit(context.Context, LegacyInFlightCompatibilityMetricLabels)
}

// LegacyInFlightCompatibilityMetricSinkFunc adapts a function to
// LegacyInFlightCompatibilityMetricSink.
type LegacyInFlightCompatibilityMetricSinkFunc func(context.Context, LegacyInFlightCompatibilityMetricLabels)

func (f LegacyInFlightCompatibilityMetricSinkFunc) Emit(ctx context.Context, labels LegacyInFlightCompatibilityMetricLabels) {
	if f == nil {
		return
	}
	f(ctx, labels)
}

// LegacyInFlightCompatibilityEventSink emits telemetry from legacy compatibility events.
//
// Implementations should avoid blocking and avoid panicking, since compatibility
// telemetry must not alter request behavior.
type LegacyInFlightCompatibilityEventSink interface {
	Emit(context.Context, LegacyInFlightCompatibilityEvent)
}

// LegacyInFlightCompatibilitySinkFunc adapts a function to
// LegacyInFlightCompatibilityEventSink.
type LegacyInFlightCompatibilitySinkFunc func(context.Context, LegacyInFlightCompatibilityEvent)

func (f LegacyInFlightCompatibilitySinkFunc) Emit(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
	if f == nil {
		return
	}
	f(ctx, event)
}

func (m *Middleware) emitLegacyInFlightCompatibility(
	ctx context.Context,
	r *http.Request,
	key string,
	outcome LegacyInFlightCompatibilityEventName,
	err error,
) {
	if m == nil || m.opts.OnLegacyInFlightCompatibility == nil || r == nil {
		return
	}
	event := LegacyInFlightCompatibilityEvent{
		Method:    strings.ToUpper(r.Method),
		Path:      requestPath(r.URL),
		Key:       legacyInFlightCompatibilityEventKey(key, m.opts.LegacyInFlightCompatibilityRawKey),
		StoreType: m.legacyRecoveryStoreType,
		Outcome:   outcome,
	}
	if err != nil {
		event.Error = err.Error()
	}
	m.opts.OnLegacyInFlightCompatibility(ctx, event)
}

func (m *Middleware) releaseLegacyInFlightWithOutcome(ctx context.Context, key string) (LegacyInFlightCompatibilityEventName, error) {
	err := m.releaseReservation(ctx, key, "")
	switch {
	case err == nil || errors.Is(err, ports.ErrLegacyInFlightReservationMissingToken):
		return LegacyInFlightCompatibilityRecovered, nil
	case errors.Is(err, ports.ErrLegacyInFlightTokenMismatch):
		return LegacyInFlightCompatibilityRejected, err
	case err != nil:
		return LegacyInFlightCompatibilityUnknown, err
	default:
		return LegacyInFlightCompatibilityRecovered, nil
	}
}

func (m *Middleware) isRecoverableLegacyInflight(record ports.IdempotencyRecord) bool {
	if m == nil || m.opts.InFlightTTL <= 0 {
		return false
	}
	if record.State != ports.IdempotencyStateInFlight {
		return false
	}
	if record.ReservationToken != "" {
		return false
	}
	if record.CreatedAt.IsZero() || m.opts.Clock == nil {
		return false
	}
	now := m.opts.Clock.Now()
	if now.Before(record.CreatedAt) {
		m.warnClockSkewRisk(record)
		return false
	}
	return now.After(record.CreatedAt.Add(m.opts.InFlightTTL))
}

func (m *Middleware) recoverLegacyInflightRecord(ctx context.Context, key string) error {
	outcome, err := m.releaseLegacyInFlightWithOutcome(ctx, key)
	if outcome != LegacyInFlightCompatibilityRecovered {
		return err
	}
	return nil
}

func legacyInFlightCompatibilityOutcome(err error) LegacyInFlightCompatibilityEventName {
	switch {
	case err == nil || errors.Is(err, ports.ErrLegacyInFlightReservationMissingToken):
		return LegacyInFlightCompatibilityRecovered
	case errors.Is(err, ports.ErrLegacyInFlightTokenMismatch):
		return LegacyInFlightCompatibilityRejected
	default:
		return LegacyInFlightCompatibilityUnknown
	}
}

func resolvedStoreType(store ports.IdempotencyStore) string {
	if store == nil {
		return "unknown"
	}
	if typ := reflect.TypeOf(store); typ != nil {
		return typ.String()
	}
	return "unknown"
}

func (m *Middleware) warnClockSkewRisk(record ports.IdempotencyRecord) {
	if m == nil {
		return
	}
	m.legacyClockSkewWarningOnce.Do(func() {
		m.opts.Logger.Warn(
			"idempotency legacy in-flight recovery is clock-skew-sensitive",
			"store_type", m.legacyRecoveryStoreType,
			"created_at", record.CreatedAt.Format(time.RFC3339),
			"now", m.opts.Clock.Now().Format(time.RFC3339),
			"clock_skew_sensitive", true,
		)
	})
}

func validateInFlightTTLAlignment(
	log ports.Logger,
	localTTL time.Duration,
	peerTTLs map[string]time.Duration,
	failOnMismatch bool,
	storeType string,
) error {
	if len(peerTTLs) == 0 || log == nil {
		return nil
	}
	names := make([]string, 0, len(peerTTLs))
	for name := range peerTTLs {
		names = append(names, name)
	}
	sort.Strings(names)
	var mismatches []string
	for _, name := range names {
		peerTTL := peerTTLs[name]
		if peerTTL <= 0 {
			continue
		}
		if peerTTL != localTTL {
			mismatches = append(mismatches, fmt.Sprintf("%s=%s", name, peerTTL))
		}
	}
	if len(mismatches) == 0 {
		return nil
	}
	log.Warn(
		"idempotency mixed-version in-flight TTL mismatch detected",
		"store_type", storeType,
		"local_ttl", localTTL.String(),
		"peer_ttls", strings.Join(mismatches, ","),
		"clock_skew_sensitive", true,
	)
	if !failOnMismatch {
		return nil
	}
	return fmt.Errorf("%w: local=%s peer_ttls=[%s]", ErrLegacyInFlightTTLMismatch, localTTL, strings.Join(mismatches, ","))
}

func validateInFlightClockPreflight(log ports.Logger, clock ports.Clock, failOnClockRisk bool, storeType string) error {
	if log == nil || clock == nil {
		return nil
	}
	first := clock.Now()
	second := clock.Now()
	if !second.Before(first) {
		return nil
	}
	log.Warn(
		"idempotency legacy in-flight clock preflight risk detected",
		"store_type", storeType,
		"first_timestamp", first.Format(time.RFC3339Nano),
		"second_timestamp", second.Format(time.RFC3339Nano),
		"clock_skew_sensitive", true,
	)
	if !failOnClockRisk {
		return nil
	}
	return fmt.Errorf(
		"%w: startup clock moved backwards across preflight window from %s to %s",
		ErrLegacyInFlightClockSkewPreflightRisk,
		first.Format(time.RFC3339Nano),
		second.Format(time.RFC3339Nano),
	)
}

func legacyInFlightCompatibilityEventKey(key string, exposeRaw bool) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return legacyInFlightCompatibilityUnknownKeyValue
	}
	if exposeRaw {
		return trimmed
	}
	h := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(h[:])
}

func legacyInFlightCompatibilityLogger(log ports.Logger) LegacyInFlightCompatibilityEventSink {
	if log == nil {
		return nil
	}
	return LegacyInFlightCompatibilitySinkFunc(func(_ context.Context, event LegacyInFlightCompatibilityEvent) {
		log.Warn(
			"idempotency legacy in-flight compatibility event",
			"method", event.Method,
			"path", event.Path,
			"key", event.Key,
			"store_type", event.StoreType,
			"outcome", string(event.Outcome),
			"error", event.Error,
		)
	})
}

func legacyInFlightCompatibilitySinksFromOptions(
	handler LegacyInFlightCompatibilityHandler,
	sink LegacyInFlightCompatibilityEventSink,
	metricSink LegacyInFlightCompatibilityMetricSink,
	log ports.Logger,
) LegacyInFlightCompatibilityEventSink {
	parts := make([]LegacyInFlightCompatibilityEventSink, 0, 3)
	if handler != nil {
		parts = append(parts, LegacyInFlightCompatibilitySinkFunc(handler))
	}
	if sink != nil {
		parts = append(parts, sink)
	}
	if metricSink != nil {
		parts = append(parts, legacyInFlightCompatibilityMetricSinkFromOptions(metricSink))
	}
	if len(parts) == 0 && log != nil {
		parts = append(parts, legacyInFlightCompatibilityLogger(log))
	}
	return legacyInFlightCompatibilityComposite(parts...)
}

func legacyInFlightCompatibilityMetricSinkFromOptions(sink LegacyInFlightCompatibilityMetricSink) LegacyInFlightCompatibilityEventSink {
	if sink == nil {
		return nil
	}
	return LegacyInFlightCompatibilitySinkFunc(func(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
		sink.Emit(ctx, event.MetricLabels())
	})
}

func legacyInFlightCompatibilityComposite(sinks ...LegacyInFlightCompatibilityEventSink) LegacyInFlightCompatibilityEventSink {
	if len(sinks) == 0 {
		return nil
	}
	if len(sinks) == 1 {
		return &legacyInFlightCompatibilitySafeSink{next: sinks[0]}
	}
	return LegacyInFlightCompatibilitySinkFunc(func(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
		for _, sink := range sinks {
			legacyInFlightCompatibilitySafeEmit(sink, ctx, event)
		}
	})
}

type legacyInFlightCompatibilitySafeSink struct {
	next LegacyInFlightCompatibilityEventSink
}

func (s legacyInFlightCompatibilitySafeSink) Emit(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
	legacyInFlightCompatibilitySafeEmit(s.next, ctx, event)
}

func legacyInFlightCompatibilitySafeEmit(sink LegacyInFlightCompatibilityEventSink, ctx context.Context, event LegacyInFlightCompatibilityEvent) {
	if sink == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	sink.Emit(ctx, event)
}

type legacyInFlightCompatibilityAsyncSink struct {
	next    LegacyInFlightCompatibilityEventSink
	log     ports.Logger
	queue   chan legacyInFlightCompatibilityAsyncEvent
	started sync.Once
	dropped atomic.Uint64
}

type legacyInFlightCompatibilityAsyncEvent struct {
	event LegacyInFlightCompatibilityEvent
}

func newLegacyInFlightCompatibilityAsyncSink(next LegacyInFlightCompatibilityEventSink, log ports.Logger) *legacyInFlightCompatibilityAsyncSink {
	if next == nil {
		return nil
	}
	return &legacyInFlightCompatibilityAsyncSink{
		next:  next,
		log:   log,
		queue: make(chan legacyInFlightCompatibilityAsyncEvent, legacyInFlightCompatibilityAsyncQueueSize),
	}
}

func (s *legacyInFlightCompatibilityAsyncSink) Emit(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
	if s == nil || s.next == nil {
		return
	}
	s.started.Do(func() {
		workerCtx := context.WithoutCancel(ctx)
		for i := 0; i < legacyInFlightCompatibilityAsyncWorkers; i++ {
			go s.drain(workerCtx)
		}
	})
	select {
	case s.queue <- legacyInFlightCompatibilityAsyncEvent{event: event}:
	default:
		dropped := s.dropped.Add(1)
		if s.log != nil {
			s.log.Warn(
				"idempotency legacy in-flight compatibility telemetry dropped",
				"dropped_events", dropped,
				"queue_size", legacyInFlightCompatibilityAsyncQueueSize,
			)
		}
	}
}

func (s *legacyInFlightCompatibilityAsyncSink) drain(ctx context.Context) {
	for item := range s.queue {
		legacyInFlightCompatibilitySafeEmit(s.next, ctx, item.event)
	}
}

type legacyInFlightCompatibilitySamplingSink struct {
	next  LegacyInFlightCompatibilityEventSink
	every int
	seen  atomic.Uint64
}

func (s *legacyInFlightCompatibilitySamplingSink) Emit(ctx context.Context, event LegacyInFlightCompatibilityEvent) {
	if s == nil || s.next == nil {
		return
	}
	if s.every <= 1 {
		s.next.Emit(ctx, event)
		return
	}
	if s.seen.Add(1)%uint64(s.every) != 0 {
		return
	}
	s.next.Emit(ctx, event)
}
