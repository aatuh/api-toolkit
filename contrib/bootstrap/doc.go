// Package bootstrap composes common contrib adapters and core middleware into application profiles.
//
// System endpoints and pprof profiles
// -----------------------------------
//
// `MountSystemEndpoints` and `MountSystemEndpointsTo` use production-safe defaults
// where pprof is disabled unless explicitly enabled.
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
// In production profiles, pass `bootstrap.SystemEndpointOptions{EnablePprof:true}` when
// there is an explicit policy decision to expose profiling.
package bootstrap
