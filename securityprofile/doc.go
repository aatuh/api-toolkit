// Package securityprofile composes stable secure middleware defaults.
//
// Profiles bundle common HTTP hardening choices such as body limits, timeouts,
// security headers, and related middleware. Applications should still review
// browser, auth, metrics, and system endpoint settings before deploying a
// profile unchanged. Use StreamingRouteOverride for streaming, SSE, websocket,
// or large-download routes that must opt out of timeout buffering.
package securityprofile
