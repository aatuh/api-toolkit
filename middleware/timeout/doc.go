// Package timeout provides cooperative request-deadline middleware and an
// explicit hard timeout variant for handlers that need a synthesized timeout
// response when downstream code ignores context cancellation.
//
// Hard timeout buffers the handler response before committing it to the client
// so late writes can be discarded after the deadline. It is not suitable for
// streaming responses, server-sent events, websocket upgrades, or handlers that
// require optional http.ResponseWriter interfaces such as http.Flusher,
// http.Hijacker, http.Pusher, or io.ReaderFrom. Handler panics are recovered
// inside the hard-timeout goroutine: before timeout they produce deterministic
// Problem Details responses, and after timeout they are contained after the 504
// response has already won.
//
// HardTimeoutEventHooks expose bounded outcome metadata for operators without
// leaking panic values, URL paths, query strings, headers, or bodies. Use the
// hook to increment low-cardinality counters or structured logs for timeout,
// panic, and capture-overflow outcomes.
package timeout
