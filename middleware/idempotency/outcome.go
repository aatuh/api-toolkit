package idempotency

import (
	"context"
	"net/http"
	"strings"
)

// OutcomeEventName identifies the low-cardinality outcome of an idempotency decision.
type OutcomeEventName string

const (
	IdempotencyOutcomeMissingKey             OutcomeEventName = "missing_key"
	IdempotencyOutcomeInvalidRequest         OutcomeEventName = "invalid_request"
	IdempotencyOutcomeLookupFailed           OutcomeEventName = "lookup_failed"
	IdempotencyOutcomeFailOpen               OutcomeEventName = "fail_open"
	IdempotencyOutcomeConflict               OutcomeEventName = "conflict"
	IdempotencyOutcomeReplayed               OutcomeEventName = "replayed"
	IdempotencyOutcomeInFlight               OutcomeEventName = "in_flight"
	IdempotencyOutcomeAmbiguous              OutcomeEventName = "ambiguous"
	IdempotencyOutcomeReservationFailed      OutcomeEventName = "reservation_failed"
	IdempotencyOutcomeReservationUnavailable OutcomeEventName = "reservation_unavailable"
	IdempotencyOutcomeCompletedStored        OutcomeEventName = "completed_stored"
	IdempotencyOutcomeCompletedReleased      OutcomeEventName = "completed_released"
	IdempotencyOutcomeResponseTooLarge       OutcomeEventName = "response_too_large"
	IdempotencyOutcomePersistenceFailed      OutcomeEventName = "persistence_failed"
)

// OutcomeEvent contains bounded idempotency outcome fields suitable for logs or
// metric labels. It intentionally omits paths, keys, tenants, request IDs, body
// data, and raw error strings.
type OutcomeEvent struct {
	Method    string
	Status    int
	StoreType string
	Outcome   OutcomeEventName
	FailOpen  bool
}

// MetricLabels returns the canonical low-cardinality label set for idempotency
// outcome metrics.
func (event OutcomeEvent) MetricLabels() map[string]string {
	return map[string]string{
		"method":       legacyInFlightCompatibilityMetricMethod(event.Method),
		"store_class":  legacyInFlightCompatibilityMetricStoreClass(event.StoreType),
		"outcome":      outcomeMetricName(event.Outcome),
		"status_class": outcomeStatusClass(event.Status),
	}
}

// OutcomeHandler receives bounded idempotency outcome events.
type OutcomeHandler func(context.Context, OutcomeEvent)

func (m *Middleware) emitOutcome(ctx context.Context, r *http.Request, outcome OutcomeEventName, status int, failOpen bool) {
	if m == nil || m.opts.OnOutcome == nil {
		return
	}
	if ctx == nil {
		return
	}
	event := OutcomeEvent{
		Status:    status,
		StoreType: m.legacyRecoveryStoreType,
		Outcome:   outcome,
		FailOpen:  failOpen,
	}
	if r != nil {
		event.Method = strings.ToUpper(strings.TrimSpace(r.Method))
	}
	defer func() {
		if recover() != nil && m.opts.Logger != nil {
			m.opts.Logger.Warn("idempotency outcome event handler panicked")
		}
	}()
	m.opts.OnOutcome(ctx, event)
}

func outcomeMetricName(outcome OutcomeEventName) string {
	switch outcome {
	case IdempotencyOutcomeMissingKey,
		IdempotencyOutcomeInvalidRequest,
		IdempotencyOutcomeLookupFailed,
		IdempotencyOutcomeFailOpen,
		IdempotencyOutcomeConflict,
		IdempotencyOutcomeReplayed,
		IdempotencyOutcomeInFlight,
		IdempotencyOutcomeAmbiguous,
		IdempotencyOutcomeReservationFailed,
		IdempotencyOutcomeReservationUnavailable,
		IdempotencyOutcomeCompletedStored,
		IdempotencyOutcomeCompletedReleased,
		IdempotencyOutcomeResponseTooLarge,
		IdempotencyOutcomePersistenceFailed:
		return string(outcome)
	default:
		return "unknown"
	}
}

func outcomeStatusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "none"
	}
}
