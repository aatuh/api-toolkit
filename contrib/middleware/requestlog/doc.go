// Package requestlog provides contrib HTTP request logging middleware.
//
// Header redaction
// ---------------
//
// The default redaction set covers common credential headers and common token /
// session header families. Matching is case-insensitive and also handles common
// underscore variants.
//
// Use WithRedactedHeaders for service-specific header keys that should always be
// redacted. Explicit WithRedactedHeaders entries still apply in addition to the
// defaults and can force redaction for non-default names.
//
// Payload field redaction contract
// -------------------------------
//
// Request wrappers that add ad-hoc log payload fields should apply the sensitive
// payload helper before logging.
//
//	fields := map[string]any{
//		"request_id": reqID,
//		"api_token":  token,
//		"password":   secret,
//	}
//
//	logger.Info("http", "fields", requestlog.RedactPayloadFields(fields))
//
// The helper treats canonical and prefixed/suffixed forms of common sensitive
// names as sensitive by default.
//
// Use RedactPayloadFieldsDeep for nested payload structures:
//
//	fields := map[string]any{
//		"request_id": reqID,
//		"metadata": map[string]any{
//			"session": map[string]any{
//				"token": token,
//			},
//		},
//	}
//
//	logger.Info("http", "fields", requestlog.RedactPayloadFieldsDeep(fields))
//
// Redaction is shape-aware for commonly used logger payload containers:
// `map[string]any`, `map[string]string`, `[]map[string]any`, and
// `[]map[string]string`. If payload builders emit custom container types,
// normalize into a supported structure before calling either helper.

// Panic logging
// ------------
//
// Recovered panics are always emitted at error level. Panic observations use
// failure status metadata (5xx) in log fields while preserving the committed
// status in `committed_status` when a response had already started.
// Enable `With5xxStackLogging(true)` to capture stack traces for recovered panics.
package requestlog
