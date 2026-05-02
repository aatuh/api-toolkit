// Package maxbody provides stable request-body size limiting middleware.
//
// Use New with an explicit byte limit before handlers read request bodies,
// especially upload, webhook, and JSON write endpoints. Oversized requests fail
// at the edge instead of allowing unbounded memory or disk pressure.
package maxbody
