// Package devheaders provides convenience wiring for development-only auth headers.
//
// This wrapper delegates to contrib/middleware/auth/devheaders and environment
// loading. Enable it only with explicit dangerous-bypass and trusted-proxy
// configuration; it is not production authentication.
package devheaders
