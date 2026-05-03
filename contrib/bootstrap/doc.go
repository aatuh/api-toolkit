// Package bootstrap composes common contrib adapters and core middleware into application profiles.
//
// System endpoints and pprof profiles
// -----------------------------------
//
// `MountSystemEndpoints` and `MountSystemEndpointsTo` preserve v2 convenience
// behavior where pprof is disabled unless explicitly enabled. Prefer
// `MountSystemEndpointsToWithAdmin` for new wiring that includes detailed
// health, pprof, or metrics so operator-only routes require an explicit
// authorization or internal-network wrapper.
//
// Development wiring example:
//
//	bootstrap.MountSystemEndpointsToWithProfile(router, bootstrap.SystemEndpoints{
//		Health:  health.NewHandler(nil),
//		Pprof:   pprof.Handler(),
//		Version: version.NewHandler(version.Config{}),
//	}, "development")
//
// Production wiring example:
//
//	bootstrap.MountSystemEndpointsToWithProfile(router, bootstrap.SystemEndpoints{
//		Health: health.NewHandler(nil),
//		Pprof:  pprof.Handler(),
//	}, "production")
//
// Admin wiring example:
//
//	err := bootstrap.MountSystemEndpointsToWithAdmin(router, bootstrap.SystemEndpoints{
//		Health:  healthHandler,
//		Metrics: bootstrap.PrometheusMetricsHandler(),
//		Pprof:   pprof.Handler(),
//	}, bootstrap.SystemEndpointAdminOptions{
//		RequireAdmin: requireAdmin,
//		EnablePprof:  true,
//	})
//	if err != nil {
//		return err
//	}
//
// In production profiles, pass `bootstrap.SystemEndpointOptions{EnablePprof:true}` when
// there is an explicit policy decision to expose profiling.
package bootstrap
