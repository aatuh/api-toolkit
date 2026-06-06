// Package compatkit provides experimental downstream compatibility checks for
// services that adopt api-toolkit runtime contracts.
//
// Use it from a consuming service's Go tests to run a named suite against an
// in-process http.Handler or an explicitly configured base URL. The package is
// designed for compatibility evidence: stable HTTP responses, Problem Details,
// OpenAPI compatibility, and other service-level checks that should keep
// passing across toolkit upgrades.
//
// Purpose: Run downstream service compatibility checks without depending on
// generated scaffold internals.
// Import: `github.com/aatuh/api-toolkit/v3/compatkit`.
// Example: See compatkit/example_test.go and docs/downstream-compatibility.md.
// Errors: RunChecks returns structured findings; Run fails the supplied test
// when any finding is present.
// Concurrency: Suite values are read during a run and should be treated as
// immutable after construction. Each check gets a fresh request context.
// Stability: Experimental test-support package, not part of the v3 stable API
// gate unless promoted through the stable API review board process.
// When not to use: Prefer package-local tests or contracttest helpers directly
// when the service does not need reusable HTTP-level compatibility evidence.
package compatkit
