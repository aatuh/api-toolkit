// Package timeout provides cooperative request-deadline middleware and an
// explicit hard timeout variant for handlers that need a synthesized timeout
// response when downstream code ignores context cancellation.
//
// Hard timeout buffers the handler response before committing it to the client
// so late writes can be discarded after the deadline. It is not suitable for
// streaming responses, server-sent events, websocket upgrades, or handlers that
// require optional http.ResponseWriter interfaces such as http.Flusher,
// http.Hijacker, http.Pusher, or io.ReaderFrom.
package timeout
