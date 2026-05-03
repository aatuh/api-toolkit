package routepolicy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/specs"
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
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := recorder.Header().Get("Deprecation"); got != "@0" {
		t.Fatalf("Deprecation = %q, want @0", got)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
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
