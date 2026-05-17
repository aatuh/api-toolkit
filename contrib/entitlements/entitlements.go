package entitlements

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Feature names an application-owned capability or quota dimension.
type Feature string

// Quota describes an optional usage limit for a feature.
type Quota struct {
	Limit int64
}

// Plan describes the features and quotas assigned to a tenant.
type Plan struct {
	ID       string
	Features map[Feature]bool
	Quotas   map[Feature]Quota
}

// Usage records usage after a quota increment.
type Usage struct {
	Used  int64
	Limit int64
}

// Store loads tenant plans and persists usage counters.
type Store interface {
	PlanForTenant(ctx context.Context, tenantID string) (Plan, error)
	IncrementUsage(ctx context.Context, tenantID string, feature Feature, amount int64) (Usage, error)
}

// DecisionReason is a low-cardinality entitlement decision reason.
type DecisionReason string

const (
	// ReasonAllowed means the feature or quota check passed.
	ReasonAllowed DecisionReason = "allowed"
	// ReasonFeatureDenied means the tenant plan does not include the feature.
	ReasonFeatureDenied DecisionReason = "feature_denied"
	// ReasonQuotaExceeded means usage crossed the configured quota limit.
	ReasonQuotaExceeded DecisionReason = "quota_exceeded"
)

var (
	// ErrInvalidCheck reports a malformed tenant, feature, or amount.
	ErrInvalidCheck = errors.New("invalid entitlement check")
	// ErrNilStore reports that no store was configured.
	ErrNilStore = errors.New("entitlement store is required")
)

// Decision is safe to send to logs or metrics because it excludes tenant IDs,
// customer IDs, subscription IDs, and other provider-owned identifiers.
type Decision struct {
	Allowed bool
	Feature Feature
	Reason  DecisionReason
	Used    int64
	Limit   int64
}

// Observer receives low-cardinality entitlement decisions.
type Observer interface {
	ObserveEntitlementDecision(ctx context.Context, decision Decision)
}

// Service enforces plan features and quotas through a Store.
type Service struct {
	Store    Store
	Observer Observer
}

// Allowed checks whether the tenant plan includes feature.
func (s Service) Allowed(ctx context.Context, tenantID string, feature Feature) (Decision, error) {
	if err := validateCheck(tenantID, feature, 1); err != nil {
		return Decision{}, err
	}
	if s.Store == nil {
		return Decision{}, ErrNilStore
	}
	plan, err := s.Store.PlanForTenant(ctx, tenantID)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{
		Allowed: plan.Features[feature],
		Feature: feature,
		Reason:  ReasonAllowed,
	}
	if !decision.Allowed {
		decision.Reason = ReasonFeatureDenied
	}
	s.observe(ctx, decision)
	return decision, nil
}

// Consume increments usage for feature and reports whether the tenant remains
// within the configured quota. Features without a positive quota are allowed
// when the plan includes the feature.
func (s Service) Consume(ctx context.Context, tenantID string, feature Feature, amount int64) (Decision, error) {
	if err := validateCheck(tenantID, feature, amount); err != nil {
		return Decision{}, err
	}
	if s.Store == nil {
		return Decision{}, ErrNilStore
	}
	plan, err := s.Store.PlanForTenant(ctx, tenantID)
	if err != nil {
		return Decision{}, err
	}
	if !plan.Features[feature] {
		decision := Decision{Allowed: false, Feature: feature, Reason: ReasonFeatureDenied}
		s.observe(ctx, decision)
		return decision, nil
	}
	quota := plan.Quotas[feature]
	if quota.Limit <= 0 {
		decision := Decision{Allowed: true, Feature: feature, Reason: ReasonAllowed}
		s.observe(ctx, decision)
		return decision, nil
	}
	usage, err := s.Store.IncrementUsage(ctx, tenantID, feature, amount)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{
		Allowed: usage.Used <= quota.Limit,
		Feature: feature,
		Reason:  ReasonAllowed,
		Used:    usage.Used,
		Limit:   quota.Limit,
	}
	if !decision.Allowed {
		decision.Reason = ReasonQuotaExceeded
	}
	s.observe(ctx, decision)
	return decision, nil
}

func (s Service) observe(ctx context.Context, decision Decision) {
	if s.Observer != nil {
		s.Observer.ObserveEntitlementDecision(ctx, decision)
	}
}

// TenantExtractor returns the tenant ID for an HTTP request.
type TenantExtractor func(*http.Request) (string, bool)

// Middleware enforces a feature before invoking the next HTTP handler.
func Middleware(service Service, extract TenantExtractor, feature Feature) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if extract == nil {
				writeProblem(w, http.StatusForbidden, "entitlement denied")
				return
			}
			tenantID, ok := extract(r)
			if !ok {
				writeProblem(w, http.StatusForbidden, "entitlement denied")
				return
			}
			decision, err := service.Allowed(r.Context(), tenantID, feature)
			if err != nil || !decision.Allowed {
				writeProblem(w, http.StatusForbidden, "entitlement denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClonePlan returns a defensive copy of plan maps.
func ClonePlan(plan Plan) Plan {
	clone := Plan{
		ID:       plan.ID,
		Features: make(map[Feature]bool, len(plan.Features)),
		Quotas:   make(map[Feature]Quota, len(plan.Quotas)),
	}
	for feature, enabled := range plan.Features {
		clone.Features[feature] = enabled
	}
	for feature, quota := range plan.Quotas {
		clone.Quotas[feature] = quota
	}
	return clone
}

func validateCheck(tenantID string, feature Feature, amount int64) error {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(string(feature)) == "" || amount <= 0 {
		return ErrInvalidCheck
	}
	return nil
}

func writeProblem(w http.ResponseWriter, status int, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": status,
	})
}
