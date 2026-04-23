## Executive summary
This repository is in good overall condition. The package layout is disciplined, the documentation is unusually strong for a library, and the shipped quality baseline is better than average: `make test`, `make test-race`, `go vet ./...`, and `go vet ./...` in `contrib/` all passed during this review.

The main material risk is not broad code health; it is a small number of hidden edge-case failures in high-value infrastructure packages. The most important one is the idempotency middleware: its retry-state cleanup is correct in the happy path, but it is not resilient to request cancellation and it exposes an extension contract that is safer in the bundled stores than it is for third-party stores.

Architecture is mostly hexagonal, but only partially adapter-neutral in practice. The core/contrib split is real and useful, yet the stable `ports` surface still carries documented Stripe-shaped billing types and pgx-shaped database stats. That keeps the design serviceable today, but it weakens the long-term “stable generic core” claim and raises future extraction cost.

Operability is also mostly solid: health, request logging, metrics, panic recovery, and middleware composition are all thoughtfully designed. The notable exception is Prometheus registration handling, where incompatible collector registration can fail silently instead of surfacing a hard error.

I did not run `make finalize`, `golangci-lint`, `govulncheck`, `gosec`, or `make api-check`. `api-check` depends on `apidiff`, which was not installed in the current environment.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  7/10 | Strong core/contrib split, but the stable core still exposes provider-shaped billing and driver-shaped stats surfaces. |
| SOLID / cohesion / coupling            |  8/10 | Packages are generally small and cohesive; the main weakness is duplicated security-sensitive auth flow. |
| Correctness & robustness               |  6/10 | Happy-path behavior is strong, but idempotency failure semantics have real cancellation and extension-point holes. |
| Security                               |  8/10 | Trusted-proxy gating, auth defaults, and SSRF controls are good; no obvious high-severity auth bypass surfaced in reviewed code. |
| Test effectiveness                     |  8/10 | Broad, behavior-oriented unit coverage plus race tests; gaps remain around cancellation, contract mismatch, and registration conflict paths. |
| Change safety & backward compatibility |  7/10 | Docs and API discipline are good, but stable surface sprawl and compatibility-sensitive ports raise future change cost. |
| Operability & observability            |  7/10 | Health/logging/metrics are well-considered overall, but metrics registration failure handling is weak. |
| Clarity & developer experience         |  8/10 | README, architecture docs, and examples are strong; duplicated wrappers and partial boundary leaks add cognitive overhead. |
| Extensibility                          |  7/10 | Ports-and-adapters structure helps, but some extension points are less safe than their interfaces suggest. |
| Overall                                |  7/10 | Good production-oriented library with a few meaningful correctness and boundary issues worth prioritizing. |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- `middleware/idempotency` performs reservation cleanup and ambiguous-state persistence on the request context instead of a bounded cleanup context (`middleware/idempotency/idempotency.go:154`, `middleware/idempotency/idempotency.go:257-301`, `middleware/idempotency/idempotency.go:307-341`). If the client disconnects or a timeout cancels `r.Context()`, `Release` and `Save` can fail and leave keys stranded in `in-flight`, `unknown`, or stale states until TTL expiry. The repository already handles cleanup writes out-of-band in similar reliability-sensitive code paths (`contrib/adapters/txpostgres/txpostgres.go:60-67`, `scheduler/scheduler.go:138-160`), so this is an inconsistency in a payment-like retry boundary rather than an acceptable pattern.

### Medium
- The idempotency store contract is internally inconsistent: `ports.IdempotencyStore` does not require release semantics (`ports/idempotency.go:34-43`), but `middleware/idempotency` effectively requires them to reopen keys after non-stored outcomes. When `Release` is absent, `releaseReservation` writes `StateUnknown` (`middleware/idempotency/idempotency.go:307-329`), and the next request then fails `TryBegin` and falls through to `writeReservationStateUnavailable` (`middleware/idempotency/idempotency.go:213-254`). The bundled stores implement `Release`, so the defect is hidden in current tests, but the public extension point is unsafe for future store implementations.
- `DefaultHash` hashes `r.URL.RawQuery` verbatim (`middleware/idempotency/idempotency.go:344-376`). Two semantically identical requests with reordered query parameters produce different hashes and are treated as conflicting reuse of the same idempotency key. That makes retry behavior dependent on client/query serialization details instead of request meaning.
- `contrib/middleware/metrics.NewPrometheusRecorder` suppresses registration failures that are not `AlreadyRegistered` and still returns a collector instance (`contrib/middleware/metrics/metrics.go:92-142`). In an application that already registered conflicting metric descriptors, the recorder can appear healthy while its metrics are not actually registered, which is a silent operability failure.
- The stable core boundary is only partially hexagonal in practice. `ports/billing.go` exposes Stripe-shaped checkout and billing vocabulary (`ports/billing.go:9-235`), and `ports/database.go` still exposes pgx-style pool stats (`ports/database.go:8-129`). The documentation acknowledges this, but downstream applications still compile against provider/driver terms inside the stable core, which weakens boundary cleanliness and increases future extraction cost.

### Low
- Security-sensitive JWT and Clerk middleware are near-copy implementations with separate constructors, config types, and request-handling logic (`middleware/auth/jwt/middleware.go:21-260`, `contrib/middleware/auth/clerk/middleware.go:21-260`). Shared helpers reduce some drift, but future fixes to token parsing, skip-header semantics, or claim requirements still need mirrored edits across two stacks.
- `endpoints/docs.Handler.Middleware` is not nil-safe (`endpoints/docs/handlers.go:119-133`), while sibling middleware adapters in the same repository do guard nil receivers (`endpoints/health/handlers.go:238-245`). This is an edge-case bug rather than a dominant runtime risk, but public middleware adapters should behave consistently.

## Hexagonal architecture verdict
State:
- Clean: the repository has a real inward dependency story. `ports`, `httpx`, `middleware`, and `endpoints` stay broadly framework-light, while `contrib/adapters/*` and `contrib/bootstrap` carry integration and wiring concerns. The docs accurately explain the intended shape, and the package map is easy to navigate.
- Leaks across boundaries: the biggest leaks are explicitly documented compatibility surfaces inside `ports`, especially billing and database stats. Those are not accidental, but they still mean the stable core is not fully generic. There is also some public-surface duplication in `contrib/integrations/*`, which widens the API without adding much behavior.
- Verdict: partially hexagonal. The codebase is much closer to ports-and-adapters than to framework-centric layering, but it is not a pure hexagonal boundary because parts of the stable core still carry provider and driver vocabulary.

## Test verdict
State:
- Covered well: core middleware behavior, HTTP helpers, auth middleware, health endpoints, contrib adapters, and many replay/error paths are all exercised. The passing `make test` and `make test-race` runs materially increase confidence here.
- Weak areas: cancellation-aware cleanup in idempotency, third-party/custom idempotency store conformance, canonical query hashing, Prometheus registration conflicts, and nil-receiver parity are not covered by the current suite.
- Verdict: confidence-building, not superficial. The tests are meaningful and broad, but they are strongest on current implementations and weaker on extension contracts and failure modes that only appear when contexts are canceled or applications compose the library with conflicting registries.

## Best next fixes
- Fix idempotency cleanup to use a bounded cleanup context independent of `r.Context()` and add regression tests for canceled-request cleanup.
- Make idempotency release semantics explicit and enforceable, either by promoting release into the required store contract or redesigning the middleware state machine so non-releaser stores are safe.
- Canonicalize idempotency hashing for query parameters and add regression tests for reordered-but-equivalent requests.
- Change Prometheus recorder construction to surface incompatible registration failures instead of swallowing them.
- Continue shrinking provider/driver-shaped stable `ports` surfaces, starting with a concrete deprecation or extraction plan for billing and legacy database stats.
- Refactor JWT and Clerk middleware onto a single shared validation pipeline with provider-specific claim mapping hooks.

## Optional follow-up
- A dependency-ordered remediation backlog has been written to `.audits/codebase_audit_v17/remediation_backlog.md`.
- A focused follow-up pass on security-sensitive middleware and extension contracts would be high value after the idempotency fixes land.
