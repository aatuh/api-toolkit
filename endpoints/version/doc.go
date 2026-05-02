// Package version registers a stable JSON version endpoint.
//
// NewHandler accepts a Config with path and ports.VersionInfo values. Register
// the handler on a router that supports method-based route registration, or use
// Handler directly when composing with net/http.
package version
