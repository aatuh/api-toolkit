package routepolicy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/specs"
)

func TestPolicyDerivesDeprecationNegotiationAndExtension(t *testing.T) {
	policy := New(Config{EnableDeprecation: true, EnableNegotiation: true, EmitPolicyExtension: true})
	operation := specs.Operation{
		Deprecated: true,
		Responses: map[int]specs.Response{
			http.StatusOK: {Content: map[string]specs.MediaType{"application/json": {Schema: map[string]any{"type": "object"}}}},
		},
	}
	updated, middleware, err := policy.Apply(operation)
	if err != nil {
		t.Fatalf("apply policy: %v", err)
	}
	if len(middleware) != 2 {
		t.Fatalf("middleware count = %d, want 2", len(middleware))
	}
	if updated.Extensions["x-api-toolkit-policy"] == nil {
		t.Fatalf("policy extension missing: %#v", updated.Extensions)
	}

	handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if got := recorder.Header().Get("Deprecation"); got != "@0" {
		t.Fatalf("Deprecation = %q, want @0", got)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/plain")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotAcceptable)
	}
}

func TestPolicyUsesAuthHookForSecuredOperations(t *testing.T) {
	called := false
	policy := New(Config{Auth: func(operation specs.Operation) (func(http.Handler) http.Handler, error) {
		called = true
		return func(next http.Handler) http.Handler { return next }, nil
	}})
	_, middleware, err := policy.Apply(specs.Operation{Scopes: []string{"widgets:read"}})
	if err != nil {
		t.Fatalf("apply policy: %v", err)
	}
	if !called || len(middleware) != 1 {
		t.Fatalf("auth hook called=%v middleware=%d", called, len(middleware))
	}
}

func TestMetadataHelpersApplyStablePolicyExtensions(t *testing.T) {
	operation := ApplyMetadata(specs.Operation{Method: http.MethodPost, Path: "/widgets"},
		WithOperationID("createWidget"),
		WithAuth("ApiKeyAuth", "widgets:write"),
		WithDeprecated(),
		WithSunset("Wed, 01 Jul 2026 00:00:00 GMT"),
		WithTenantRequired("header"),
		WithIdempotencyRequired(),
		WithRateLimit("write-standard"),
		WithAdminPolicy("operators"),
		WithProblemResponses(http.StatusBadRequest, http.StatusConflict),
	)

	if operation.OperationID != "createWidget" {
		t.Fatalf("operation id = %q", operation.OperationID)
	}
	if len(operation.Security) != 1 || operation.Security[0].Name != "ApiKeyAuth" {
		t.Fatalf("security = %#v", operation.Security)
	}
	if !operation.Deprecated {
		t.Fatal("deprecated flag was not set")
	}
	if operation.Sunset != "Wed, 01 Jul 2026 00:00:00 GMT" {
		t.Fatalf("sunset = %q", operation.Sunset)
	}
	if _, ok := operation.Extensions[ExtensionTenant]; !ok {
		t.Fatalf("tenant extension missing: %#v", operation.Extensions)
	}
	if _, ok := operation.Extensions[ExtensionIdempotencyKey]; !ok {
		t.Fatalf("idempotency extension missing: %#v", operation.Extensions)
	}
	tenant, ok := TenantPolicyFromOperation(operation)
	if !ok || !tenant.Required || tenant.Source != "header" {
		t.Fatalf("tenant policy = %#v ok=%v", tenant, ok)
	}
	idempotency, ok := IdempotencyPolicyFromOperation(operation)
	if !ok || !idempotency.Required || idempotency.Header != "Idempotency-Key" {
		t.Fatalf("idempotency policy = %#v ok=%v", idempotency, ok)
	}
	deprecation, ok := DeprecationPolicyFromOperation(operation)
	if !ok || !deprecation.Deprecated || deprecation.Sunset != "Wed, 01 Jul 2026 00:00:00 GMT" {
		t.Fatalf("deprecation policy = %#v ok=%v", deprecation, ok)
	}
	auth, ok := AuthPolicyFromOperation(operation)
	if !ok || len(auth.Security) != 1 || len(auth.Scopes) != 1 || auth.Scopes[0] != "widgets:write" {
		t.Fatalf("auth policy = %#v ok=%v", auth, ok)
	}
	rateLimit, ok := RateLimitPolicyFromOperation(operation)
	if !ok || rateLimit != "write-standard" {
		t.Fatalf("rate limit = %q ok=%v", rateLimit, ok)
	}
	adminPolicy, ok := AdminPolicyFromOperation(operation)
	if !ok || adminPolicy != "operators" {
		t.Fatalf("admin policy = %q ok=%v", adminPolicy, ok)
	}
	problems := ProblemResponseStatuses(operation)
	if len(problems) != 2 || problems[0] != http.StatusBadRequest || problems[1] != http.StatusConflict {
		t.Fatalf("problem response statuses = %#v", problems)
	}
	if got := operation.Extensions[ExtensionRateLimit]; got != "write-standard" {
		t.Fatalf("rate limit extension = %#v", got)
	}
	if _, ok := operation.Responses[http.StatusBadRequest]; !ok {
		t.Fatalf("bad request response missing: %#v", operation.Responses)
	}
}

func TestObservabilityLabelsFromOperationAreBounded(t *testing.T) {
	operation := ApplyMetadata(specs.Operation{Method: http.MethodPost, Path: "/widgets"},
		WithAuth("ApiKeyAuth", "widgets:write"),
		WithTenantRequired("tenant-from-request"),
		WithIdempotencyRequired(),
		WithRateLimit("per-customer-write-burst"),
		WithAdminPolicy("platform-admins"),
		WithDeprecated(),
	)

	labels := ObservabilityLabelsFromOperation(operation)
	if labels.Auth != "required" {
		t.Fatalf("auth label = %q", labels.Auth)
	}
	if labels.Tenant != "required" {
		t.Fatalf("tenant label = %q", labels.Tenant)
	}
	if labels.Idempotency != "required" {
		t.Fatalf("idempotency label = %q", labels.Idempotency)
	}
	if labels.RateLimit != "configured" {
		t.Fatalf("rate-limit label = %q", labels.RateLimit)
	}
	if labels.Admin != "required" {
		t.Fatalf("admin label = %q", labels.Admin)
	}
	if labels.Deprecated != "true" {
		t.Fatalf("deprecated label = %q", labels.Deprecated)
	}
	for key, value := range labels.Map() {
		if strings.Contains(value, "customer") || strings.Contains(value, "platform") || strings.Contains(value, "tenant-from-request") {
			t.Fatalf("label %s leaked raw policy value %q", key, value)
		}
	}

	empty := ObservabilityLabelsFromOperation(specs.Operation{})
	if empty.Auth != "none" || empty.Tenant != "none" || empty.Idempotency != "none" || empty.RateLimit != "none" || empty.Admin != "none" || empty.Deprecated != "false" {
		t.Fatalf("empty labels = %#v", empty)
	}
}

func TestLintOperationsFindsMissingProductionPolicy(t *testing.T) {
	findings := LintOperations([]specs.Operation{{
		Method:    http.MethodPost,
		Path:      "/widgets",
		Responses: map[int]specs.Response{http.StatusCreated: {}},
	}}, LintOptions{
		RequireOperationID:                true,
		RequireUnsafeWriteAuth:            true,
		RequireUnsafeWriteIdempotency:     true,
		RequireUnsafeWriteProblemResponse: true,
	})

	for _, code := range []string{"operation_id_required", "unsafe_write_auth_required", "unsafe_write_idempotency_required", "problem_response_required"} {
		if !hasFinding(findings, code) {
			t.Fatalf("missing finding %s in %#v", code, findings)
		}
	}
}

func TestLintOperationsFindsDuplicateOperationID(t *testing.T) {
	findings := LintOperations([]specs.Operation{
		{
			OperationID: "getWidget",
			Method:      http.MethodGet,
			Path:        "/widgets/{id}",
		},
		{
			OperationID: "getWidget",
			Method:      http.MethodGet,
			Path:        "/widget-exports/{id}",
		},
	}, LintOptions{RequireUniqueOperationID: true})

	if !hasFinding(findings, "operation_id_duplicate") {
		t.Fatalf("missing duplicate operation ID finding in %#v", findings)
	}
}

func TestLintOperationsFindsNonPublicOperationWithoutSecurity(t *testing.T) {
	findings := LintOperations([]specs.Operation{
		{
			OperationID: "listWidgets",
			Method:      http.MethodGet,
			Path:        "/widgets",
			Responses:   map[int]specs.Response{http.StatusOK: {}},
		},
		{
			OperationID: "ready",
			Method:      http.MethodGet,
			Path:        "/readyz",
			Responses:   map[int]specs.Response{http.StatusOK: {}},
		},
	}, LintOptions{
		RequireSecurity: true,
		PublicPaths:     []string{"/readyz"},
	})

	if !hasFinding(findings, "security_required") {
		t.Fatalf("missing security finding in %#v", findings)
	}
	if len(findings) != 1 {
		t.Fatalf("expected only private route finding, got %#v", findings)
	}
}

func TestLintOperationsFindsNonPublicOperationWithoutProblemResponse(t *testing.T) {
	findings := LintOperations([]specs.Operation{
		{
			OperationID: "listWidgets",
			Method:      http.MethodGet,
			Path:        "/widgets",
			Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:read"}}},
			Responses:   map[int]specs.Response{http.StatusOK: {}},
		},
		{
			OperationID: "ready",
			Method:      http.MethodGet,
			Path:        "/readyz",
			Responses:   map[int]specs.Response{http.StatusOK: {}},
		},
	}, LintOptions{
		RequireProblemResponse: true,
		PublicPaths:            []string{"/readyz"},
	})

	if !hasFinding(findings, "problem_response_required") {
		t.Fatalf("missing problem response finding in %#v", findings)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, want only private operation finding", findings)
	}
}

func TestLintOperationsFindsUnsafeWriteMissingTenantAndRateLimit(t *testing.T) {
	findings := LintOperations([]specs.Operation{{
		OperationID: "createWidget",
		Method:      http.MethodPost,
		Path:        "/widgets",
		Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
		Responses: map[int]specs.Response{
			http.StatusCreated:    {},
			http.StatusBadRequest: specs.ProblemResponse("Bad Request"),
		},
		Extensions: map[string]any{
			ExtensionIdempotencyKey: map[string]any{"required": true},
		},
	}}, LintOptions{
		RequireUnsafeWriteTenant:    true,
		RequireUnsafeWriteRateLimit: true,
	})

	for _, code := range []string{"unsafe_write_tenant_required", "unsafe_write_rate_limit_required"} {
		if !hasFinding(findings, code) {
			t.Fatalf("missing finding %s in %#v", code, findings)
		}
	}
}

func TestLintOperationsFindsUnsafeWriteNonRequiredTenantAndIdempotency(t *testing.T) {
	findings := LintOperations([]specs.Operation{{
		OperationID: "createWidget",
		Method:      http.MethodPost,
		Path:        "/widgets",
		Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
		Responses: map[int]specs.Response{
			http.StatusCreated:    {},
			http.StatusBadRequest: specs.ProblemResponse("Bad Request"),
		},
		RequestBody: &specs.RequestBody{
			Required: true,
			Content:  map[string]specs.MediaType{"application/json": {}},
		},
		Extensions: map[string]any{
			ExtensionTenant:         map[string]any{"required": false, "source": "header"},
			ExtensionIdempotencyKey: map[string]any{"required": false, "header": "Idempotency-Key"},
			ExtensionRateLimit:      "write-standard",
		},
	}}, LintOptions{
		RequireUnsafeWriteTenant:      true,
		RequireUnsafeWriteIdempotency: true,
	})

	for _, code := range []string{"unsafe_write_tenant_required", "unsafe_write_idempotency_required"} {
		if !hasFinding(findings, code) {
			t.Fatalf("missing finding %s in %#v", code, findings)
		}
	}
}

func TestLintOperationsFindsUnsafeWriteInvalidRateLimitPolicy(t *testing.T) {
	findings := LintOperations([]specs.Operation{{
		OperationID: "createWidget",
		Method:      http.MethodPost,
		Path:        "/widgets",
		Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
		Responses: map[int]specs.Response{
			http.StatusCreated:    {},
			http.StatusBadRequest: specs.ProblemResponse("Bad Request"),
		},
		RequestBody: &specs.RequestBody{
			Required: true,
			Content:  map[string]specs.MediaType{"application/json": {}},
		},
		Extensions: map[string]any{
			ExtensionTenant:         map[string]any{"required": true, "source": "header"},
			ExtensionIdempotencyKey: map[string]any{"required": true, "header": "Idempotency-Key"},
			ExtensionRateLimit:      map[string]any{"policy": "write-standard"},
		},
	}}, LintOptions{RequireUnsafeWriteRateLimit: true})

	if !hasFinding(findings, "unsafe_write_rate_limit_required") {
		t.Fatalf("missing rate-limit finding in %#v", findings)
	}
}

func TestLintOperationsFindsUndocumentedUnsafeWrite(t *testing.T) {
	findings := LintOperations([]specs.Operation{
		{
			OperationID: "createWidget",
			Method:      http.MethodPost,
			Path:        "/widgets",
			Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
			Responses: map[int]specs.Response{
				http.StatusBadRequest: specs.ProblemResponse("Bad Request"),
			},
			Extensions: map[string]any{
				ExtensionTenant:         map[string]any{"required": true},
				ExtensionIdempotencyKey: map[string]any{"required": true},
				ExtensionRateLimit:      "write-standard",
			},
		},
		{
			OperationID: "deleteWidget",
			Method:      http.MethodDelete,
			Path:        "/widgets/{id}",
			Security:    []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: []string{"widgets:write"}}},
			Responses: map[int]specs.Response{
				http.StatusNoContent:  {},
				http.StatusBadRequest: specs.ProblemResponse("Bad Request"),
			},
			Extensions: map[string]any{
				ExtensionTenant:         map[string]any{"required": true},
				ExtensionIdempotencyKey: map[string]any{"required": true},
				ExtensionRateLimit:      "write-standard",
			},
		},
	}, LintOptions{
		RequireUnsafeWriteRequestBody:     true,
		RequireUnsafeWriteSuccessResponse: true,
	})

	for _, code := range []string{"unsafe_write_request_body_required", "unsafe_write_success_response_required"} {
		if !hasFinding(findings, code) {
			t.Fatalf("missing finding %s in %#v", code, findings)
		}
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %#v, want only POST request-body and success-response findings", findings)
	}
}

func TestLintOperationsFindsAdminRouteWithoutPolicy(t *testing.T) {
	findings := LintOperations([]specs.Operation{{
		OperationID: "readProfile",
		Method:      http.MethodGet,
		Path:        "/debug/pprof/",
		Responses:   map[int]specs.Response{http.StatusOK: {}, http.StatusBadRequest: specs.ProblemResponse("")},
	}}, LintOptions{AdminPaths: []string{"/debug/pprof/"}})

	if !hasFinding(findings, "admin_policy_required") {
		t.Fatalf("missing admin policy finding in %#v", findings)
	}
}

func hasFinding(findings []LintFinding, code string) bool {
	for _, finding := range findings {
		if strings.EqualFold(finding.Code, code) {
			return true
		}
	}
	return false
}
