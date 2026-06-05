package bootstrap_test

import (
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/contrib/v3/bootstrap"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

type exampleSystemEndpointRouter struct {
	patterns []string
}

func (r *exampleSystemEndpointRouter) Get(pattern string, _ http.HandlerFunc) {
	r.patterns = append(r.patterns, pattern)
}

func ExampleMountSystemEndpointsToWithAdmin() {
	router := &exampleSystemEndpointRouter{}
	healthHandler := health.NewBasicHandler()
	requireAdmin := func(next http.Handler) http.Handler { return next }

	if err := configureSystemEndpoints(router, healthHandler, requireAdmin); err != nil {
		panic(err)
	}

	fmt.Println(len(router.patterns) > 0)

	// Output:
	// true
}

func configureSystemEndpoints(router *exampleSystemEndpointRouter, healthHandler *health.Handler, requireAdmin func(http.Handler) http.Handler) error {
	err := bootstrap.MountSystemEndpointsToWithAdmin(router, bootstrap.SystemEndpoints{
		Health:  healthHandler,
		Metrics: bootstrap.PrometheusMetricsHandler(),
		Pprof:   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	}, bootstrap.SystemEndpointAdminOptions{
		RequireAdmin: requireAdmin,
		EnablePprof:  true,
	})
	if err != nil {
		return err
	}
	return nil
}
