## Executive summary
This audit covers the full repository as it exists on April 19, 2026. Review evidence came from repository structure inspection, focused reads of the highest-centrality packages, and local quality checks: `make test`, `make lint`, `make test-race`, and `make api-check`.

The codebase is still in better shape than a typical toolkit repository of this size. The `ports` package stays small, adapters mostly depend inward, and the HTTP middleware packages are readable and intentionally scoped. The strongest architectural property is that most third-party coupling is kept in `contrib/*` instead of leaking through core packages.

The biggest current risks are concentrated in shared HTTP primitives rather than in the larger package structure. `response_writer.Writer` and `response_writer.Capture` are used as infrastructure for recovery, request logging, metrics, idempotency, and OpenAPI response validation. Their current behavior does not fully preserve `net/http` response semantics, so one low-level bug can fan out across multiple higher-level packages.

The other notable weakness is output hardening around the docs surface. `endpoints/docs` still builds HTML and inline JS with direct string interpolation from config values, which is a poor default for a public-facing toolkit surface even when the immediate source is "just config". There is also a smaller exported-API sharp edge in `contrib/config.Loader`, whose zero value is not safe despite looking like a normal helper type.

The repo’s tests are broad and generally useful, and lint/race checks are currently green. The remaining gap is that several cross-package contracts are only lightly exercised: response commitment semantics, buffered replay semantics, and docs page escaping are all examples where leaf-package green tests do not prove integration safety.

## Scorecard
| Dimension                              | Score | Notes                                                                                                                                             |
|----------------------------------------|------:|---------------------------------------------------------------------------------------------------------------------------------------------------|
| Architecture & boundaries              |  8/10 | Core `ports` and adapter split is still clean; most leakage is in HTTP composition helpers rather than business boundaries.                       |
| SOLID / cohesion / coupling            |  7/10 | Packages are mostly focused, but shared response primitives have too much blast radius for their current test coverage.                           |
| Correctness & robustness               |  6/10 | Shared response wrappers do not fully match `net/http` commitment rules, which can skew recovery, logging, metrics, and buffered replay behavior. |
| Security                               |  7/10 | JWT and SSRF hardening are solid, but docs HTML generation still interpolates raw config into HTML and inline JS.                                 |
| Test effectiveness                     |  7/10 | Broad unit coverage and green race/lint runs; weaker around cross-package response semantics and output escaping.                                 |
| Change safety & backward compatibility |  8/10 | API checks are present and passing; risks are more behavioral than exported-API related.                                                          |
| Operability & observability            |  6/10 | Metrics/request logging are thoughtfully placed, but they depend on response tracking that can currently report the wrong visible status.         |
| Clarity & developer experience         |  7/10 | Package layout is readable, though a few exported helpers have surprising edge behavior.                                                          |
| Extensibility                          |  8/10 | Adding adapters or middleware remains relatively cheap because the repo keeps interfaces and implementations separated well.                      |
| Overall                                |  7/10 | Good structural base with a small number of real correctness and hardening issues worth fixing immediately.                                       |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- `response_writer.Writer` does not preserve real response commitment semantics across `WriteHeader`, `Flush`, and panic recovery paths. `WriteHeader` always overwrites the tracked status, and `Flush` does not mark the response as committed at all ([response_writer/wrapper.go](/home/aatu/projects/saas/api-toolkit/response_writer/wrapper.go:61), [response_writer/wrapper.go](/home/aatu/projects/saas/api-toolkit/response_writer/wrapper.go:84)). That tracked state is then consumed by recovery, request logging, and metrics ([httpx/recover/recover.go](/home/aatu/projects/saas/api-toolkit/httpx/recover/recover.go:78), [contrib/middleware/requestlog/requestlog.go](/home/aatu/projects/saas/api-toolkit/contrib/middleware/requestlog/requestlog.go:163), [contrib/middleware/metrics/metrics.go](/home/aatu/projects/saas/api-toolkit/contrib/middleware/metrics/metrics.go:179)). Result: flushed streaming responses can be misclassified as uncommitted, and repeated `WriteHeader` calls can produce the wrong visible status in logs and metrics.

### Medium
- `response_writer.Capture` also overwrites status on repeated `WriteHeader` and exposes its internal body slice despite documenting a copy return ([response_writer/capture.go](/home/aatu/projects/saas/api-toolkit/response_writer/capture.go:42), [response_writer/capture.go](/home/aatu/projects/saas/api-toolkit/response_writer/capture.go:81)). That buffer feeds OpenAPI response validation and idempotency replay state ([contrib/middleware/openapi/openapi.go](/home/aatu/projects/saas/api-toolkit/contrib/middleware/openapi/openapi.go:180), [middleware/idempotency/idempotency.go](/home/aatu/projects/saas/api-toolkit/middleware/idempotency/idempotency.go:264)). A downstream handler that accidentally double-writes a status can therefore be replayed or validated against a response that differs from standard `net/http` behavior.
- Docs HTML generation interpolates config values directly into HTML text, attributes, and inline JavaScript via `fmt.Sprintf` ([endpoints/docs/docs.go](/home/aatu/projects/saas/api-toolkit/endpoints/docs/docs.go:216)). This is config-driven rather than request-driven, so it is not an immediate remote exploit surface, but it is still the wrong default for a public docs endpoint because malformed or hostile metadata can break the page or inject script.

### Low
- `contrib/config.Loader` is not zero-value safe. `Require`, `String`, `Duration`, and `CSV` dereference `l.env` directly, while `raw()` already contains nil guards ([contrib/config/loader.go](/home/aatu/projects/saas/api-toolkit/contrib/config/loader.go:25)). For an exported helper type, that inconsistency is a developer-experience footgun and makes defensive reuse harder than it needs to be.

## Hexagonal architecture verdict
What is clean:
- `ports` remains a genuinely useful inward-facing contract layer.
- Most vendor-specific code is held in `contrib/adapters/*` and `contrib/integrations/*`.
- Core middleware packages are small and do not drag framework types deeper than necessary.

What leaks across boundaries:
- `contrib/bootstrap` concentrates a large amount of default HTTP policy and effectively becomes a framework opinion surface.
- Shared HTTP response wrappers in `response_writer` sit underneath multiple unrelated concerns, so behavioral mistakes there leak across recovery, logging, metrics, OpenAPI validation, and idempotency.

Verdict:
- The codebase is partially hexagonal, leaning strong on ports-and-adapters in the core while still being fairly HTTP-framework-centric in its outer composition layer.

## Test verdict
What is covered well:
- Package-level unit tests are widespread.
- The standard suite, lint, race run, and API compatibility check all pass locally.
- Security-sensitive areas such as JWT parsing, SSRF guards, and several middleware packages already have useful direct tests.

What is weak:
- Shared response semantics are under-tested relative to their blast radius.
- There are no direct regressions for flush-before-panic commitment handling, repeated `WriteHeader` semantics, or docs HTML escaping.
- Some confidence currently comes from leaf packages being green even when the important risk is in how packages compose together.

Verdict:
- Tests are confidence-building for most isolated packages, but not yet strong enough around the response pipeline contracts that other packages rely on.

## Best next fixes
1. Make `response_writer.Writer` preserve first final status and mark flush-driven commitment correctly.
2. Make `response_writer.Capture` preserve first final status and return an immutable body snapshot.
3. Replace docs HTML string interpolation with templated escaping for HTML and inline JS contexts.
4. Make `contrib/config.Loader` safe to use from its zero value.

## Optional follow-up
- Package-by-package review of `response_writer`, `httpx`, and the HTTP composition layer.
- Security-focused pass over other config-derived output surfaces.
- Test-gap plan for cross-package middleware interactions.
