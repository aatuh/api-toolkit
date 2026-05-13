// Package clerk provides Clerk JWT middleware.
//
// The authenticated Subject includes tenant and scope values derived from
// validated token claims so applications can enforce policy after Clerk JWT
// validation. When enabled, JWKS refresh runs in the background. Call
// Middleware.Close() or cancel the context passed to NewMiddleware to stop the
// refresh goroutine.
package clerk
