// Package timeout provides cooperative request-deadline middleware and an
// explicit hard timeout variant for finite routes that need a synthesized
// timeout response when downstream code ignores context cancellation.
//
// Use NewPropagator globally. It adds a cooperative deadline without buffering
// the response or changing optional http.ResponseWriter capabilities. For a
// finite JSON route that needs a hard wall-clock cutoff, create NewHard and use
// WrapRoute with RouteCapabilityFiniteJSON. WrapRoute fails closed for streaming
// responses, server-sent events, websocket upgrades, large downloads, and
// handlers that require http.Flusher, http.Hijacker, http.Pusher, or
// io.ReaderFrom.
//
// Hard timeout starts one handler goroutine for each wrapped request and buffers
// its complete response before committing it. MaxCaptureBytes defaults to 1 MiB
// when unset. It cannot stop CPU work that ignores ctx.Done; use
// HardTimeoutEventHooks.OnHandlerContinues for a bounded signal when the timeout
// response is committed while that goroutine is still running. Handler panics
// are recovered inside the child goroutine: before timeout they produce
// deterministic Problem Details responses, and after timeout they are
// contained after the 504 response has already won.
//
// HardTimeoutEventHooks expose bounded outcome metadata for operators without
// leaking panic values, URL paths, query strings, headers, or bodies. Use the
// hook to increment low-cardinality counters or structured logs for timeout,
// panic, capture-overflow, and handler-continuation outcomes.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/middleware/timeout`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package timeout
