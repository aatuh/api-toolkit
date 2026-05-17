package entitlements

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceAllowedDeniesMissingFeature(t *testing.T) {
	store := &memoryStore{plan: Plan{
		Features: map[Feature]bool{"widgets": true},
	}}
	observer := &recordingObserver{}
	service := Service{Store: store, Observer: observer}

	decision, err := service.Allowed(context.Background(), "tenant-1", "imports")
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if decision.Allowed || decision.Reason != ReasonFeatureDenied {
		t.Fatalf("Allowed() decision = %#v", decision)
	}
	if len(observer.decisions) != 1 || observer.decisions[0].Reason != ReasonFeatureDenied {
		t.Fatalf("observer decisions = %#v", observer.decisions)
	}
}

func TestServiceConsumeTracksQuota(t *testing.T) {
	store := &memoryStore{plan: Plan{
		Features: map[Feature]bool{"widgets": true},
		Quotas:   map[Feature]Quota{"widgets": {Limit: 2}},
	}}
	service := Service{Store: store}

	decision, err := service.Consume(context.Background(), "tenant-1", "widgets", 1)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if !decision.Allowed || decision.Used != 1 || decision.Limit != 2 {
		t.Fatalf("Consume() decision = %#v", decision)
	}
	decision, err = service.Consume(context.Background(), "tenant-1", "widgets", 2)
	if err != nil {
		t.Fatalf("Consume() second error = %v", err)
	}
	if decision.Allowed || decision.Reason != ReasonQuotaExceeded || decision.Used != 3 {
		t.Fatalf("Consume() second decision = %#v", decision)
	}
}

func TestServiceDoesNotIncrementDeniedFeature(t *testing.T) {
	store := &memoryStore{plan: Plan{Features: map[Feature]bool{"widgets": true}}}
	service := Service{Store: store}

	decision, err := service.Consume(context.Background(), "tenant-1", "imports", 1)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if decision.Allowed || store.used != 0 {
		t.Fatalf("Consume() decision = %#v used=%d", decision, store.used)
	}
}

func TestMiddlewareRejectsWithoutLeakingTenant(t *testing.T) {
	service := Service{Store: &memoryStore{plan: Plan{Features: map[Feature]bool{"widgets": false}}}}
	handler := Middleware(service, func(*http.Request) (string, bool) {
		return "tenant-secret", true
	}, "widgets")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "tenant-secret") {
		t.Fatalf("response leaked tenant ID: %s", recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content-type = %q", got)
	}
}

func TestClonePlanDefensiveCopy(t *testing.T) {
	plan := Plan{
		ID:       "pro",
		Features: map[Feature]bool{"widgets": true},
		Quotas:   map[Feature]Quota{"widgets": {Limit: 10}},
	}
	clone := ClonePlan(plan)
	clone.Features["widgets"] = false
	clone.Quotas["widgets"] = Quota{Limit: 1}

	if !plan.Features["widgets"] || plan.Quotas["widgets"].Limit != 10 {
		t.Fatalf("original plan mutated: %#v", plan)
	}
}

type memoryStore struct {
	plan Plan
	used int64
}

func (s *memoryStore) PlanForTenant(context.Context, string) (Plan, error) {
	return ClonePlan(s.plan), nil
}

func (s *memoryStore) IncrementUsage(_ context.Context, _ string, _ Feature, amount int64) (Usage, error) {
	s.used += amount
	return Usage{Used: s.used}, nil
}

type recordingObserver struct {
	decisions []Decision
}

func (o *recordingObserver) ObserveEntitlementDecision(_ context.Context, decision Decision) {
	o.decisions = append(o.decisions, decision)
}
