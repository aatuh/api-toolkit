// Package idempotency provides idempotency utilities.
// The middleware buffers responses before replay/storage and is therefore not
// suitable for streaming, hijacking, HTTP/2 push, or other handlers that rely
// on optional http.ResponseWriter interfaces. If a completed response cannot be
// persisted for replay, the middleware fails closed with 503 and stores an
// ambiguous state for that key instead of reopening it for another execution.
// Buffered responses that exceed Options.MaxResponseBytes follow the same
// ambiguous-outcome path. The default request hash includes authenticated actor
// and tenant scope when earlier middleware has populated them in request
// context.
package idempotency
