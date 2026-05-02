// Package httpclient provides outbound HTTP client adapters with retry-aware defaults.
//
// Retry defaults
// -------------
//
// By default, retry is enabled only for safe read-only methods:
//
// - GET
// - HEAD
//
// Caller code should add additional methods only when explicit idempotency
// guarantees exist for the upstream contract.
//
// Use RetryableMethods to opt in, for example to allow PUT retries when you know
// every retried payload is safe to replay:
//
//	client := httpclient.New(httpclient.Options{
//		Retry: httpclient.RetryOptions{
//			MaxRetries:       2,
//			RetryableMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut},
//		},
//	})
//
// Retry migration path
// -------------------
//
// If you previously retried non-idempotent methods broadly, migrate in phases:
//
// 1) Roll out with defaults (GET/HEAD only) and confirm request-level SLIs.
// 2) Add method opt-in only where upstream call semantics are explicitly replay-safe.
//
// Example:
//
//	client := httpclient.New(httpclient.Options{
//		Retry: httpclient.RetryOptions{
//			MaxRetries: 2,
//			RetryableMethods: []string{
//				http.MethodGet,
//				http.MethodHead,
//				// Enable PUT only after upstream replay semantics are reviewed.
//				http.MethodPut,
//			},
//			UseRetryAfter: true,
//		},
//	})
//
// Retry budgets
// -------------
//
// Configure max budget controls for backoff and Retry-After parsing:
//
//	client := httpclient.New(httpclient.Options{
//		Retry: httpclient.RetryOptions{
//			MaxRetries:    5,
//			MaxElapsedTime: 2 * time.Second,
//			MinBackoff:    100 * time.Millisecond,
//			MaxBackoff:    2 * time.Second,
//			UseRetryAfter: true,
//		},
//	})
//
// A retry loop stops when elapsed delay would exceed MaxElapsedTime.
package httpclient
