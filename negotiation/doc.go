// Package negotiation provides stable core HTTP content negotiation helpers.
//
// Use New when a service wants one middleware to enforce both Accept and
// Content-Type policy, or RequireAccept and RequireContentType for focused
// checks. ParseAccept, Negotiate, and ContentTypeAllowed expose the same matching
// behavior for handlers and tests.
//
// The package writes Problem Details 406 and 415 responses and does not inspect
// request bodies, route metadata, or OpenAPI documents. Keep media-type policy
// explicit in application routing or route contracts.
package negotiation
