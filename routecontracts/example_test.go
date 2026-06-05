package routecontracts_test

import (
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/v3/routecontracts"
	"github.com/aatuh/api-toolkit/v3/specs"
)

type exampleRouteRouter struct {
	calls []string
}

func (r *exampleRouteRouter) Get(pattern string, _ http.HandlerFunc) {
	r.calls = append(r.calls, http.MethodGet+" "+pattern)
}
func (r *exampleRouteRouter) Post(pattern string, _ http.HandlerFunc) {
	r.calls = append(r.calls, http.MethodPost+" "+pattern)
}
func (r *exampleRouteRouter) Put(pattern string, _ http.HandlerFunc) {
	r.calls = append(r.calls, http.MethodPut+" "+pattern)
}
func (r *exampleRouteRouter) Delete(pattern string, _ http.HandlerFunc) {
	r.calls = append(r.calls, http.MethodDelete+" "+pattern)
}

func ExampleRegistry_Get() {
	router := &exampleRouteRouter{}
	openapi := specs.NewRegistry(specs.Info{Title: "Widget API", Version: "1.0.0"})
	registry := routecontracts.NewRegistry(router, openapi)

	if err := registry.Get("/widgets", specs.Operation{
		OperationID: "listWidgets",
		Responses:   map[int]specs.Response{http.StatusOK: {Description: "OK"}},
	}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err != nil {
		panic(err)
	}

	fmt.Println(router.calls[0])

	// Output:
	// GET /widgets
}
