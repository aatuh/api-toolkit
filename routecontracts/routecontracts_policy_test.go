package routecontracts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/specs"
)

type policyTestRouter struct {
	handlers map[string]http.HandlerFunc
}

func (r *policyTestRouter) Get(pattern string, h http.HandlerFunc) { r.set(http.MethodGet, pattern, h) }
func (r *policyTestRouter) Post(pattern string, h http.HandlerFunc) {
	r.set(http.MethodPost, pattern, h)
}
func (r *policyTestRouter) Put(pattern string, h http.HandlerFunc) { r.set(http.MethodPut, pattern, h) }
func (r *policyTestRouter) Delete(pattern string, h http.HandlerFunc) {
	r.set(http.MethodDelete, pattern, h)
}

func (r *policyTestRouter) set(method, pattern string, h http.HandlerFunc) {
	if r.handlers == nil {
		r.handlers = map[string]http.HandlerFunc{}
	}
	r.handlers[method+" "+pattern] = h
}

func TestRegistryPolicyMiddlewareIsWrappedByRouteMiddleware(t *testing.T) {
	router := &policyTestRouter{}
	registry := NewRegistryWithOptions(router, nil, Options{Policies: []Policy{PolicyFunc(func(operation specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error) {
		operation.Extensions = map[string]any{"x-policy": true}
		return operation, []func(http.Handler) http.Handler{appendOrder("policy")}, nil
	})}})

	err := registry.Get("/widgets", specs.Operation{}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Order", "handler")
	}, appendOrder("route"))
	if err != nil {
		t.Fatalf("register route: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.handlers["GET /widgets"].ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets", nil))
	if got := recorder.Header().Values("X-Order"); len(got) != 3 || got[0] != "route" || got[1] != "policy" || got[2] != "handler" {
		t.Fatalf("middleware order = %v", got)
	}
	routes := registry.Routes()
	if len(routes) != 1 || routes[0].Operation.Extensions["x-policy"] != true {
		t.Fatalf("policy operation extension not retained: %#v", routes)
	}
}

func TestRegistryPolicyErrorStopsRegistration(t *testing.T) {
	expected := errors.New("boom")
	router := &policyTestRouter{}
	registry := NewRegistryWithOptions(router, nil, Options{Policies: []Policy{PolicyFunc(func(operation specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error) {
		return operation, nil, expected
	})}})

	err := registry.Get("/widgets", specs.Operation{}, func(http.ResponseWriter, *http.Request) {})
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v, want %v", err, expected)
	}
	if len(router.handlers) != 0 {
		t.Fatalf("route was registered after policy error")
	}
}

func appendOrder(value string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Order", value)
			next.ServeHTTP(w, r)
		})
	}
}
