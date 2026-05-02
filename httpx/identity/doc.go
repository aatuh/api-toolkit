// Package identity resolves canonical request identity values.
//
// Resolver extracts client IP, scheme, host, and request ID from an HTTP request.
// Forwarded headers are honored only when the direct peer matches configured
// trusted proxies, which keeps proxy-derived identity explicit and safe by
// default.
package identity
