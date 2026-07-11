package pprof_test

import (
	"fmt"
	"net/http"

	pprofendpoint "github.com/aatuh/api-toolkit/v4/endpoints/pprof"
)

type examplePprofRouter struct {
	patterns []string
}

func (r *examplePprofRouter) Get(pattern string, _ http.HandlerFunc) {
	r.patterns = append(r.patterns, pattern)
}

func ExampleRegisterAdminRoutes() {
	router := &examplePprofRouter{}
	requireAdmin := func(next http.Handler) http.Handler { return next }

	if err := pprofendpoint.RegisterAdminRoutes(router, requireAdmin); err != nil {
		panic(err)
	}

	fmt.Println(len(router.patterns))

	// Output:
	// 5
}
