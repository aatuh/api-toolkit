// Package recover provides panic recovery utilities for HTTP handlers.
//
// Contract:
//   - if the response is still uncommitted, a recovered panic becomes a 500
//     Problem Details response
//   - if headers or body bytes have already been committed, the middleware logs
//     the panic and aborts the request instead of preserving a misleading
//     partial success response
package recover
