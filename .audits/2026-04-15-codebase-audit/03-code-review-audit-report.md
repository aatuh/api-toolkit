# Code Review Audit Report

## Executive summary
This repository has a credible ports-and-adapters shape. The `core` and `contrib` split is understandable, dependency direction is mostly inward, and the packages are small enough to review quickly. It is a stronger foundation than a typical utility-heavy Go toolkit.

The biggest problems are not abstract architecture issues. They are runtime and contract issues concentrated at the HTTP boundary and in composition layers: bootstrap defaults, error handling, request validation, and public examples. Those areas are exactly where reuse risk is highest, because downstream services will inherit them as policy.

The codebase also overperforms its current tests in the wrong direction: low-level units are covered, but several central paths are not. That allowed concrete bugs to survive in `httpx/recover`, bootstrap metrics initialization, docs handler construction, and JSON validation semantics despite green test and race runs.

The strongest aspect is extensibility. Adding adapters or swapping infrastructure remains relatively cheap because the ports are narrow and packages are cohesive. The weakest aspect is change safety: the code, docs, and examples do not yet define one stable, boring HTTP contract that consumers can trust.

Review scope covered the whole repository with focus on `ports`, `httpx`, `middleware`, `endpoints`, `securityprofile`, `contrib/bootstrap`, `contrib/middleware/*`, and selected adapters. Verification run locally included `make test`, `make test-race`, `go test ./...`, and targeted temporary repros outside the repo for suspected defects.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  7/10 | Core/contrib split is mostly clean; boundary ownership of HTTP contract is still diffuse. |
| SOLID / cohesion / coupling            |  7/10 | Packages are focused and small; a few constructors and middleware defer misconfiguration to runtime. |
| Correctness & robustness               |  5/10 | Multiple evidence-backed defects exist in central runtime paths. |
| Security                               |  6/10 | JWT and SSRF posture is decent; auth semantics and validation behavior remain inconsistent. |
| Test effectiveness                     |  5/10 | Good low-level coverage, weak composition and trust-boundary coverage. |
| Change safety & backward compatibility |  5/10 | Examples, docs, and helpers teach incompatible contracts. |
| Operability & observability            |  6/10 | Logging, tracing, and metrics foundations are useful; failure correlation and default metrics initialization need work. |
| Clarity & developer experience         |  7/10 | Docs and package map are good; some docs overstate behavior or omit caveats. |
| Extensibility                          |  7/10 | Ports and adapters keep future integrations cheap. |
| Overall                                |  6/10 | Good foundation with several priority correctness and contract issues. |

Confidence: medium

## Findings by severity
### Critical
- None.

### High
- Default metrics wiring can panic on repeated initialization because `contrib/middleware/metrics/metrics.go:91` registers global collectors each time, and bootstrap calls it from `contrib/bootstrap/profile.go:161`, `contrib/bootstrap/profile.go:252`, and `contrib/bootstrap/http.go:47`. A temporary repro confirmed the duplicate-registration panic.
- Panic recovery corrupts already-started responses. `httpx/recover/recover.go:14` always writes a Problem response after panic, even if the handler already committed headers or body.
- JSON validation is both too permissive and too inconsistent. `middleware/json/json.go:73` accepts invalid media types like `text/application/json`, and `middleware/json/json.go:46` emits plain-text `415` instead of Problem Details.
- Authn and authz status semantics are inconsistent between `middleware/auth/jwt`, `middleware/auth/authz`, and `middleware/auth/tenant`. Missing identity, missing roles, and missing tenant scope do not map cleanly to `401` versus `403`.
- The spec-first public example documents an error shape that does not match actual runtime behavior. `contrib/examples/spec-first/openapi.json:25` and `contrib/examples/spec-first/openapi.json:112` advertise `{code,message}` while runtime middleware emits Problem Details with validation extensions.

### Medium
- Timeout behavior is documented as a resource-consumption control in `README.md:374` and `docs/security.md:44`, but `middleware/timeout/timeout.go:10` only applies a context deadline. A slow handler that ignores `ctx.Done()` still completes normally. That is a documentation and behavior mismatch with operational implications.
- `endpoints/list/list.go:73`, `endpoints/list/list.go:148`, and `endpoints/list/list.go:205` silently coerce malformed pagination inputs and silently drop unsupported filter and sort fields. That is convenient but not a stable default REST contract.
- `ProfileStrictAPI` does not include `querylimits` in the default chain even though docs position query limits as baseline hardening. That disconnect is in `contrib/bootstrap/profile.go:206`.
- `endpoints/docs/handlers.go:17` allows nil manager construction and later panics in handler methods and middleware.
- Several important packages have no direct tests: `contrib/bootstrap`, `middleware/auth/jwt`, `httpx/recover`, `endpoints/docs`, and `scheduler`.

### Low
- Error responses do not carry request correlation data even though request logs and traces do.
- Documentation around "core" purity and default-hardening behavior is slightly stronger than the code actually guarantees.

## Hexagonal architecture verdict
What is clean:
- `ports` is the right anchor for dependency inversion.
- Most adapters depend inward and are segregated in `contrib`.
- Domain-neutral HTTP helpers are isolated from specific vendors.

What leaks across boundaries:
- HTTP contract policy is split across bootstrap, middleware, helpers, docs, and examples.
- Some exported constructors permit invalid configurations that later fail at runtime.
- Operational behavior is partially owned by examples and docs rather than by one authoritative composition layer.

This is partially hexagonal. The dependency structure is mostly right, but the externally visible behavior is still more toolkit-layered than strictly boundary-owned.

## Test verdict
What is covered well:
- Low-level middleware mechanics.
- Helper functions in `httpx`, `ratelimit`, `querylimits`, `trace`, and related packages.

What is weak:
- Bootstrap composition.
- JWT trust-boundary behavior.
- Panic and partial-write paths.
- Interface-backed endpoint handlers.

The tests are confidence-building for leaf packages, but too superficial around composition and client-visible failure modes.

## Best next fixes
1. Make metrics recorder initialization idempotent or caller-scoped, then add bootstrap tests.
2. Fix recovery middleware so a panic after partial write does not append a second payload.
3. Tighten JSON media-type validation and normalize `415` responses to Problem Details.
4. Standardize `401` versus `403` semantics across JWT, role, and tenant middleware.
5. Align the spec-first example and generated contract with actual RFC 9457 runtime errors.
6. Decide whether timeouts are advisory context deadlines or enforced response ceilings, then implement and document one behavior.
7. Add tests for `middleware/auth/jwt`, `httpx/recover`, `endpoints/docs`, and `contrib/bootstrap`.

## Optional follow-up
- Targeted remediation plan
- Package-by-package review
- PR comments draft
- Refactor roadmap
- Security-focused pass
- Test-gap plan
