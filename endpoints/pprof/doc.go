// Package pprof registers Go pprof handlers on an HTTP router.
//
// RegisterRoutes wires profiling handlers only; it does not add authentication,
// authorization, or network restrictions. Prefer RegisterAdminRoutes for new
// admin mounts so an explicit wrapper is required at construction time.
package pprof
