// Package querylimits provides stable core query-parameter guardrail middleware.
//
// Use New with Options when handlers need bounded query parameter counts, key
// lengths, value lengths, and pagination limit values before application
// parsing. Middleware and Handler expose the same guardrail for ports.Middleware
// wiring or direct net/http usage.
//
// Invalid query shapes fail with Problem Details 400 responses and do not reach
// the wrapped handler. The package does not parse business filters or database
// queries; use queryparams for typed collection-query parsing after these size
// and limit checks pass.
package querylimits
