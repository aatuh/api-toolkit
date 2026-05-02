package contracttest

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/routecontracts"
	"github.com/aatuh/api-toolkit/v2/specs"
)

func TestContractAssertionsPassForCoveredRoute(t *testing.T) {
	specRegistry := specs.NewRegistry(specs.Info{Title: "Contracts", Version: "1"})
	specRegistry.Register(specs.Operation{
		Method: http.MethodGet,
		Path:   "/widgets",
		Security: []specs.SecurityRequirement{{
			Name: "ApiKeyAuth",
		}},
		Responses: map[int]specs.Response{
			http.StatusOK: {Description: "ok"},
		},
	})
	routeRegistry := routecontracts.NewRegistry(fakeRouter{}, nil)
	if err := routeRegistry.Get("/widgets", specs.Operation{Method: http.MethodGet, Path: "/widgets"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})); err != nil {
		t.Fatalf("register route: %v", err)
	}

	AssertRegistryValid(t, routeRegistry)
	AssertRouteCoverage(t, routeRegistry, http.MethodGet, "/widgets")
	AssertOperationHasResponse(t, specRegistry, http.MethodGet, "/widgets", http.StatusOK)
	AssertOperationHasSecurity(t, specRegistry, http.MethodGet, "/widgets", "ApiKeyAuth")
	AssertProblemCatalogHas(t, httpx.DefaultProblemCatalog(), httpx.ProblemCode(httpx.TypeBadRequest))
}

func TestNormalizeAndGoldenOpenAPI(t *testing.T) {
	got, err := NormalizeOpenAPI([]byte(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatalf("NormalizeOpenAPI() error = %v", err)
	}
	want := []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("normalized = %s", got)
	}
	GoldenOpenAPI(t, []byte(`{"a":1}`), []byte(`{"a":1}`))
	if _, err := NormalizeOpenAPI([]byte(`{`)); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

type fakeRouter struct{}

func (fakeRouter) Get(pattern string, h http.HandlerFunc)    {}
func (fakeRouter) Post(pattern string, h http.HandlerFunc)   {}
func (fakeRouter) Put(pattern string, h http.HandlerFunc)    {}
func (fakeRouter) Delete(pattern string, h http.HandlerFunc) {}
