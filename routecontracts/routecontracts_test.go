package routecontracts

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/specs"
)

func TestRegistryRegistersRouteAndOperation(t *testing.T) {
	router := &fakeRouter{}
	specRegistry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	registry := NewRegistry(router, specRegistry)
	var order []string
	err := registry.Get("/widgets", specs.Operation{
		Summary: "List widgets",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}), func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1")
			next.ServeHTTP(w, r)
		})
	}, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2")
			next.ServeHTTP(w, r)
		})
	})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got := router.calls[0]; got != "GET /widgets" {
		t.Fatalf("registered route = %q", got)
	}
	router.handlers[0].ServeHTTP(nil, nil)
	if !reflect.DeepEqual(order, []string{"mw1", "mw2", "handler"}) {
		t.Fatalf("middleware order = %#v", order)
	}
	doc, err := specRegistry.OpenAPI()
	if err != nil {
		t.Fatalf("OpenAPI() error = %v", err)
	}
	if !strings.Contains(string(doc), `"/widgets"`) {
		t.Fatalf("OpenAPI = %s", doc)
	}
	if err := registry.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRegistryPatchRequiresPatchRouter(t *testing.T) {
	registry := NewRegistry(&fakeRouter{}, nil)
	err := registry.Patch("/widgets/1", specs.Operation{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if err == nil || !strings.Contains(err.Error(), "PATCH") {
		t.Fatalf("Patch() error = %v, want unsupported", err)
	}

	patchRouter := &fakePatchRouter{fakeRouter: &fakeRouter{}}
	registry = NewRegistry(patchRouter, nil)
	err = registry.Patch("/widgets/1", specs.Operation{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if got := patchRouter.calls[0]; got != "PATCH /widgets/1" {
		t.Fatalf("registered route = %q", got)
	}
}

func TestRegistryReportsUnsupportedAndInvalidRoutes(t *testing.T) {
	registry := NewRegistry(&fakeRouter{}, nil)
	if err := registry.Register(Route{Method: "TRACE", Pattern: "/", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}); err == nil {
		t.Fatal("expected unsupported method error")
	}
	if err := registry.Register(Route{Method: http.MethodGet, Pattern: "/", Operation: specs.Operation{Method: http.MethodPost, Path: "/"}, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}); err == nil {
		t.Fatal("expected operation mismatch error")
	}
}

func TestRegistryValidateReportsDuplicatesAndMissingCoverage(t *testing.T) {
	registry := &Registry{routes: []Route{
		{Method: http.MethodGet, Pattern: "/widgets", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), Operation: specs.Operation{Method: http.MethodGet, Path: "/widgets"}},
		{Method: http.MethodGet, Pattern: "/widgets", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), Operation: specs.Operation{Method: http.MethodGet, Path: "/widgets"}},
		{Method: http.MethodPost, Pattern: "/widgets", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
	}}
	err := registry.Validate()
	if err == nil {
		t.Fatal("expected coverage error")
	}
	if !strings.Contains(err.Error(), "registered more than once") || !strings.Contains(err.Error(), "missing matching operation") {
		t.Fatalf("Validate() error = %v", err)
	}
	if !strings.Contains(err.Error(), "GET /widgets") {
		t.Fatalf("Validate() error = %v", err)
	}
	var coverage *CoverageError
	if !errors.As(err, &coverage) || len(coverage.Problems) == 0 {
		t.Fatalf("coverage error = %#v", coverage)
	}
}

func TestRegistryRoutesAreDeterministic(t *testing.T) {
	registry := &Registry{routes: []Route{
		{Method: http.MethodPost, Pattern: "/widgets"},
		{Method: http.MethodGet, Pattern: "/accounts"},
		{Method: http.MethodGet, Pattern: "/widgets"},
	}}
	got := registry.Routes()
	keys := []string{got[0].Method + " " + got[0].Pattern, got[1].Method + " " + got[1].Pattern, got[2].Method + " " + got[2].Pattern}
	want := []string{"GET /accounts", "GET /widgets", "POST /widgets"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("routes = %#v, want %#v", keys, want)
	}
}

type fakeRouter struct {
	calls    []string
	handlers []http.HandlerFunc
}

func (r *fakeRouter) Get(pattern string, h http.HandlerFunc)  { r.record(http.MethodGet, pattern, h) }
func (r *fakeRouter) Post(pattern string, h http.HandlerFunc) { r.record(http.MethodPost, pattern, h) }
func (r *fakeRouter) Put(pattern string, h http.HandlerFunc)  { r.record(http.MethodPut, pattern, h) }
func (r *fakeRouter) Delete(pattern string, h http.HandlerFunc) {
	r.record(http.MethodDelete, pattern, h)
}

func (r *fakeRouter) record(method, pattern string, h http.HandlerFunc) {
	r.calls = append(r.calls, method+" "+pattern)
	r.handlers = append(r.handlers, h)
}

type fakePatchRouter struct {
	*fakeRouter
}

func (r *fakePatchRouter) Patch(pattern string, h http.HandlerFunc) {
	r.record(http.MethodPatch, pattern, h)
}
