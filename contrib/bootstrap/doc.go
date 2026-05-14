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
//
// Middleware order
// ----------------
//
// `ProfileStrictAPI` records its middleware stages in `Profile.MiddlewareOrder`.
// The default router validates that the production order keeps request IDs,
// recovery, tracing, secure headers, rate limits, body/query limits, JSON
// enforcement, timeout, request logging, and metrics in the expected sequence.
// Services that compose custom routers can use `RequireMiddlewareOrder` as a
// startup check. SaaS API services that add route-policy middleware outside the
// transport profile can use `StrictSaaSAPIMiddlewareOrder` to require auth,
// tenant, and idempotency stages after metrics.
//
// Lifecycle
// ---------
//
// `APIServiceConfig.StartupChecks` validates wiring before serving traffic.
// `APIServiceConfig.AdminAddr` starts a separate admin listener; the public
// handler keeps only public health/docs/version routes while the admin handler
// serves detailed health, metrics, and pprof behind the configured admin
// wrapper.
// `APIServiceConfig.BackgroundTasks` runs named tasks such as health refresh
// schedulers with the service context; a task error fails the service and
// triggers graceful shutdown.
// `APIServiceConfig.ShutdownHooks` releases named resources after the HTTP
// server exits, using a fresh timeout derived from the Start context with
// cancellation stripped so shutdown work can complete.
package bootstrap
