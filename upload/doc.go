// Package upload decodes multipart upload requests with validation errors.
//
// The package is stable core API. It owns transport-level multipart parsing,
// request and file size checks, required file fields, content-type allowlists,
// and Problem Details-compatible field errors. It does not own object storage,
// virus scanning, media processing, persistence, or authorization.
//
// Use DecodeMultipart with Config to parse a request into a Form, then
// RequireFile to fetch required file fields by name. AllowedContentTypes and
// MaxFileBytes are small helpers for constructing readable Config values.
// ValidationProblem and WriteValidationProblem preserve the same error shape as
// binding and httpx.
//
// Always apply authentication, authorization, total request limits, and
// application-owned scanning before trusting uploaded content. For examples, see
// docs/cookbook.md.
package upload
