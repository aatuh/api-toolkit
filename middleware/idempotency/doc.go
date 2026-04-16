// Package idempotency provides idempotency utilities.
// The middleware buffers responses before replay/storage and is therefore not
// suitable for streaming, hijacking, HTTP/2 push, or other handlers that rely
// on optional http.ResponseWriter interfaces. If a completed response cannot be
// persisted for replay, the middleware fails closed with 503 instead of
// returning an ambiguous success response. The default request hash includes
// authenticated actor and tenant scope when earlier middleware has populated
// them in request context. Buffered replay bodies are capped by
// Options.MaxResponseBytes.
package idempotency
