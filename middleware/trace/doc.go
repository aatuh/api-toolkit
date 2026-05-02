// Package trace provides stable request trace and correlation middleware.
//
// The middleware propagates request and trace identifiers through context and
// response headers so logs, Problem Details responses, and downstream calls can
// be correlated without coupling handlers to a tracing provider.
package trace
