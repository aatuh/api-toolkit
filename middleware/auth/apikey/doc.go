// Package apikey provides stable API key authentication middleware.
//
// The middleware extracts credentials from Authorization: ApiKey <secret> or
// X-API-Key, delegates verification to an application-owned Verifier, stores the
// authenticated principal in request context, and can enforce required scopes.
// Storage, hashing, rotation, and last-used tracking intentionally belong to the
// Verifier implementation instead of the core middleware.
package apikey
