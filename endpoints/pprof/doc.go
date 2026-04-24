// Package pprof provides pprof utilities.
//
// RegisterRoutes wires Go pprof handlers only; it does not add authentication,
// authorization, or network restrictions. Mount these routes behind internal or
// admin access control, and avoid exposing them on public API routers.
package pprof
