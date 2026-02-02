// Package clerk provides Clerk JWT middleware.
//
// When enabled, JWKS refresh runs in the background. Call Middleware.Close()
// or cancel the context passed to NewMiddleware to stop the refresh goroutine.
package clerk
