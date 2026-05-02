// Package pprof registers Go pprof handlers on an HTTP router.
//
// RegisterRoutes wires profiling handlers only; it does not add authentication,
// authorization, or network restrictions. Mount these routes behind internal or
// admin access control, and avoid exposing them on public API routers.
package pprof
