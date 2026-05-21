// Package httpcache provides stable core conditional request helpers for REST APIs.
//
// Start with StrongETag, WeakETag, HashETag, Validators, EvaluateRead, and
// EvaluateWrite to implement ETag and Last-Modified handling without tying a
// handler to a storage backend. SetValidators, WriteNotModified, and
// WritePreconditionFailed provide the common HTTP response path.
//
// The package only evaluates standard validators: ETag, Last-Modified,
// If-None-Match, If-Modified-Since, If-Match, and If-Unmodified-Since. Callers
// still own resource versioning, authorization, and persistence updates.
package httpcache
