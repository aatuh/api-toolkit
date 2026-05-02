// Package jwt provides convenience wiring around the stable JWT middleware.
//
// The package aliases core JWT middleware types, reads JWT_* environment
// configuration, and exposes the same subject and health-check helpers. Import
// the core middleware directly when environment loading is not needed.
package jwt
