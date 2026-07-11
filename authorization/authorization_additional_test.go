package authorization

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

type ownerResource string

func (r ownerResource) OwnerID() string { return string(r) }

type tenantResource string

func (r tenantResource) TenantID() string { return string(r) }

func TestRequireAndOwnershipHelpers(t *testing.T) {
	if err := Require(context.Background(), nil, "user", "read", nil); err == nil {
		t.Fatal("expected missing authorizer error")
	}
	allowed := AuthorizerFunc(func(context.Context, any, string, any) error { return nil })
	if err := Require(context.Background(), allowed, "user", "read", nil); err != nil {
		t.Fatalf("Require() error = %v", err)
	}
	if err := RequireOwnerID("user_1", "user_1"); err != nil {
		t.Fatalf("RequireOwnerID() error = %v", err)
	}
	for _, err := range []error{
		RequireOwnerID("", "user_1"),
		RequireOwnerID("user_1", ""),
		RequireOwnerID("user_1", "user_2"),
		RequireOwner("user_1", nil),
		RequireOwner("user_1", ownerResource("user_2")),
	} {
		if !errors.Is(err, httpx.ErrForbidden) {
			t.Fatalf("expected forbidden ownership error, got %v", err)
		}
	}
	if err := RequireOwner("user_1", ownerResource("user_1")); err != nil {
		t.Fatalf("RequireOwner() error = %v", err)
	}
}

func TestTenantFieldProjectionAndScopes(t *testing.T) {
	if err := RequireTenant("tenant_1", tenantResource("tenant_1")); err != nil {
		t.Fatalf("RequireTenant() error = %v", err)
	}
	for _, err := range []error{
		RequireTenant("", tenantResource("tenant_1")),
		RequireTenant("tenant_1", nil),
		RequireTenant("tenant_1", tenantResource("")),
		RequireTenant("tenant_1", tenantResource("tenant_2")),
	} {
		if !errors.Is(err, httpx.ErrForbidden) {
			t.Fatalf("expected forbidden tenant error, got %v", err)
		}
	}
	input := map[string]any{"id": "1", "secret": "hide", "name": "widget"}
	if got := ProjectFields(input, []string{"id", "", "name"}); !reflect.DeepEqual(got, map[string]any{"id": "1", "name": "widget"}) {
		t.Fatalf("ProjectFields() = %#v", got)
	}
	if got := MaskFields(input, []string{"secret", ""}); !reflect.DeepEqual(got, map[string]any{"id": "1", "name": "widget"}) {
		t.Fatalf("MaskFields() = %#v", got)
	}
	if ProjectFields(nil, []string{"id"}) != nil || MaskFields(nil, []string{"secret"}) != nil {
		t.Fatal("nil field maps should stay nil")
	}
	scope := Scope{TenantID: "tenant_1", UserID: "user_1"}
	if got := scope.Filters(); !reflect.DeepEqual(got, map[string]any{"tenant_id": "tenant_1", "user_id": "user_1"}) {
		t.Fatalf("Filters() = %#v", got)
	}
	if (Scope{}).Filters() != nil {
		t.Fatal("empty scope should produce nil filters")
	}
	if got := ApplyScope(map[string]any{"status": "active", "tenant_id": "old"}, scope); !reflect.DeepEqual(got, map[string]any{"status": "active", "tenant_id": "tenant_1", "user_id": "user_1"}) {
		t.Fatalf("ApplyScope() = %#v", got)
	}
	if ApplyScope(nil, Scope{}) != nil {
		t.Fatal("empty filters and empty scope should produce nil")
	}
}

func TestAuthorizationContextHelpers(t *testing.T) {
	var nilCtx context.Context
	ctx := WithScope(nilCtx, Scope{TenantID: "tenant_1", UserID: "user_1"})
	scope, ok := ScopeFromContext(ctx)
	if !ok || scope.TenantID != "tenant_1" || scope.UserID != "user_1" {
		t.Fatalf("ScopeFromContext() = %#v, %v", scope, ok)
	}
	if tenant, ok := TenantIDFromContext(ctx); !ok || tenant != "tenant_1" {
		t.Fatalf("TenantIDFromContext() = %q, %v", tenant, ok)
	}
	if _, ok := ScopeFromContext(nilCtx); ok {
		t.Fatal("nil context should not have scope")
	}
	if _, ok := TenantIDFromContext(context.Background()); ok {
		t.Fatal("empty context should not have tenant")
	}
	ctx = WithActor(nilCtx, Actor{UserID: "user_1"})
	actor, ok := ActorFromContext(ctx)
	if !ok || actor.UserID != "user_1" {
		t.Fatalf("ActorFromContext() = %#v, %v", actor, ok)
	}
	if _, ok := ActorFromContext(nilCtx); ok {
		t.Fatal("nil context should not have actor")
	}
	if _, ok := ActorFromContext(WithActor(context.Background(), Actor{})); ok {
		t.Fatal("empty actor should not be returned")
	}
}

func TestAllowlistAndPolicyErrorBranches(t *testing.T) {
	var nilAllowlist *AllowlistAuthorizer
	if err := nilAllowlist.Allow("read", AuthorizerFunc(func(context.Context, any, string, any) error { return nil })); err == nil {
		t.Fatal("expected nil allowlist error")
	}
	allowlist := NewAllowlistAuthorizer()
	if err := allowlist.Allow("read", nil); err == nil {
		t.Fatal("expected nil authorizer error")
	}
	if err := allowlist.AllowFunc("read", nil); err == nil {
		t.Fatal("expected nil authorizer func error")
	}
	if err := allowlist.Can(context.Background(), "user", " ", nil); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("blank action = %v", err)
	}
	var nilPolicy *PolicyAuthorizer
	if err := nilPolicy.Can(context.Background(), "user", "read", nil); err == nil {
		t.Fatal("expected nil policy error")
	}
	engineErr := errors.New("engine down")
	engine := &stubPolicyEngine{err: engineErr}
	policy := NewPolicyAuthorizer(engine, PolicyAuthorizerOptions{})
	if err := policy.Can(context.Background(), "user", "read", nil); !errors.Is(err, engineErr) {
		t.Fatalf("policy engine error = %v", err)
	}
	engine = &stubPolicyEngine{decision: PolicyDecision{Allow: false, Reason: "scope missing"}}
	policy = NewPolicyAuthorizer(engine, PolicyAuthorizerOptions{})
	if err := policy.Can(context.Background(), "user", "read", nil); !errors.Is(err, httpx.ErrForbidden) {
		t.Fatalf("policy denial error = %v", err)
	}
}
