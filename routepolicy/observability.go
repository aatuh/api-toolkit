package routepolicy

import (
	"context"
	"net/http"

	"github.com/aatuh/api-toolkit/v4/specs"
)

const (
	observabilityRequired   = "required"
	observabilityConfigured = "configured"
	observabilityNone       = "none"
	observabilityTrue       = "true"
	observabilityFalse      = "false"
)

type observabilityContextKey struct{}

// ObservabilityLabels is the bounded route-policy label set intended for
// request logs and metrics. It does not include raw policy names, scopes,
// tenants, users, request data, or provider identifiers.
type ObservabilityLabels struct {
	Auth        string
	Tenant      string
	Idempotency string
	RateLimit   string
	Admin       string
	Deprecated  string
}

type observabilityLabelsBox struct {
	labels ObservabilityLabels
	set    bool
}

// ObservabilityLabelsFromOperation returns bounded labels for an operation's
// policy metadata.
func ObservabilityLabelsFromOperation(operation specs.Operation) ObservabilityLabels {
	labels := ObservabilityLabels{
		Auth:        observabilityNone,
		Tenant:      observabilityNone,
		Idempotency: observabilityNone,
		RateLimit:   observabilityNone,
		Admin:       observabilityNone,
		Deprecated:  observabilityFalse,
	}
	if _, ok := AuthPolicyFromOperation(operation); ok {
		labels.Auth = observabilityRequired
	}
	if policy, ok := TenantPolicyFromOperation(operation); ok && policy.Required {
		labels.Tenant = observabilityRequired
	}
	if policy, ok := IdempotencyPolicyFromOperation(operation); ok && policy.Required {
		labels.Idempotency = observabilityRequired
	}
	if _, ok := RateLimitPolicyFromOperation(operation); ok {
		labels.RateLimit = observabilityConfigured
	}
	if _, ok := AdminPolicyFromOperation(operation); ok {
		labels.Admin = observabilityRequired
	}
	if policy, ok := DeprecationPolicyFromOperation(operation); ok && policy.Deprecated {
		labels.Deprecated = observabilityTrue
	}
	return labels
}

// Map returns labels using stable key names.
func (labels ObservabilityLabels) Map() map[string]string {
	return map[string]string{
		"auth":        normalizeObservabilityLabel(labels.Auth, observabilityNone),
		"tenant":      normalizeObservabilityLabel(labels.Tenant, observabilityNone),
		"idempotency": normalizeObservabilityLabel(labels.Idempotency, observabilityNone),
		"rate_limit":  normalizeObservabilityLabel(labels.RateLimit, observabilityNone),
		"admin":       normalizeObservabilityLabel(labels.Admin, observabilityNone),
		"deprecated":  normalizeObservabilityLabel(labels.Deprecated, observabilityFalse),
	}
}

// SeedObservabilityContext installs the mutable request-scoped label carrier
// used by outer logging and metrics middleware. It is safe to call repeatedly.
func SeedObservabilityContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(observabilityContextKey{}).(*observabilityLabelsBox); ok {
		return ctx
	}
	return context.WithValue(ctx, observabilityContextKey{}, &observabilityLabelsBox{})
}

// ObservabilityMiddleware annotates a request with bounded labels derived from
// the route operation. It is primarily installed by routecontracts.
func ObservabilityMiddleware(operation specs.Operation) func(http.Handler) http.Handler {
	labels := ObservabilityLabelsFromOperation(operation)
	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r == nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			box, ok := ctx.Value(observabilityContextKey{}).(*observabilityLabelsBox)
			if !ok || box == nil {
				box = &observabilityLabelsBox{}
				ctx = context.WithValue(ctx, observabilityContextKey{}, box)
				r = r.WithContext(ctx)
			}
			box.labels = labels
			box.set = true
			next.ServeHTTP(w, r)
		})
	}
}

// ObservabilityLabelsFromRequest returns labels attached to the current route.
func ObservabilityLabelsFromRequest(r *http.Request) (ObservabilityLabels, bool) {
	if r == nil {
		return ObservabilityLabels{}, false
	}
	box, ok := r.Context().Value(observabilityContextKey{}).(*observabilityLabelsBox)
	if !ok || box == nil || !box.set {
		return ObservabilityLabels{}, false
	}
	return box.labels, true
}

func normalizeObservabilityLabel(value, fallback string) string {
	switch value {
	case observabilityRequired, observabilityConfigured, observabilityNone, observabilityTrue, observabilityFalse:
		return value
	default:
		return fallback
	}
}
