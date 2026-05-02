// Package devheaders provides development-only debug auth header middleware.
//
// The middleware is disabled by default and requires explicit dangerous-bypass
// opt-in plus trusted-proxy configuration before it honors debug identity
// headers. Do not use it as production authentication.
package devheaders
