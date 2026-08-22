// Package timeout provides cooperative request-deadline middleware and an
// explicit hard timeout variant for handlers that need a synthesized timeout
// response when downstream code ignores context cancellation.
//
// Hard timeout starts one child goroutine and buffers the handler response up
// to MaxCaptureBytes before committing it to the client, so late writes can be
// discarded after the deadline. It is not suitable for streaming responses,
// server-sent events, websocket upgrades, large downloads, or handlers that
// require optional http.ResponseWriter interfaces such as http.Flusher,
// http.Hijacker, http.Pusher, or io.ReaderFrom. Apply cooperative Propagator
// middleware globally. For a finite route that needs a wall-clock cutoff, use
// HardTimeout.WrapRoute with declared RouteCapabilities. Handler panics are
// recovered inside the hard-timeout goroutine: before timeout they produce
// deterministic Problem Details responses, and after timeout they are
// contained after the 504 response has already won.
//
// HardTimeoutEventHooks expose bounded outcome metadata for operators without
// leaking panic values, URL paths, query strings, headers, or bodies. Use the
// hook to increment low-cardinality counters or structured logs for timeout,
// panic, and capture-overflow outcomes. OnHandlerContinuesAfterTimeout is a
// separate low-level signal for work still running after the timeout response.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/middleware/timeout`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package timeout
