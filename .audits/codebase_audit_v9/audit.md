## Executive summary
This audit covers the full `api-toolkit` repository: the root `v2` module plus the `contrib/v2` module, with emphasis on the highest-centrality packages (`endpoints/health`, `middleware/idempotency`, `contrib/adapters/httpclient`, `contrib/migrator`, `scheduler`, and bootstrap helpers). The codebase is materially stronger than the average utility toolkit: the ports-and-adapters split is mostly consistent, the repo carries meaningful tests around risky middleware behavior, and the shipped build contract is unusually explicit.

The main risks are not broad architectural collapse or missing hygiene. They are contract bugs in public runtime paths that survive because the baseline checks are green and the edge cases are only hit under specific failure modes. The three confirmed defects are all in behaviorally sensitive areas: health reporting, outbound retry behavior, and idempotent write protection.

The strongest aspect of the repository is that the intended architecture is visible in code, not just in docs. `ports` is stable and dependency-light, contrib adapters mostly depend inward, and the repo already uses targeted tests to lock in behavior around auth, rate limiting, migration safety, and panic recovery. That makes the codebase changeable.

The biggest gap is change safety around edge-case contracts. Several packages document strict guarantees, but a few implementations diverge from those guarantees in uncommon paths: `GetDetailedHealth` skips the configured timeout, `httpclient.Client` can replace the real upstream failure with a local replayability error, and the idempotency middleware can continue processing after losing a reservation race even when the store cannot confirm state. Those are fixable without redesign, but they are real correctness issues.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  8/10 | `ports` remains clean and adapters mostly depend inward; a few contrib/bootstrap helpers still mix library code with process control via `OrExit` helpers, but the main dependency flow is sound. |
| SOLID / cohesion / coupling            |  7/10 | Most packages are focused, but several hotspots (`middleware/idempotency`, `endpoints/health/checkers`, `contrib/migrator`) have grown into large multi-responsibility units that raise review and extension cost. |
| Correctness & robustness               |  6/10 | Core paths are tested and baseline checks are green, but confirmed contract bugs exist in retry safety, detailed health timeouts, and idempotency race handling. |
| Security                               |  8/10 | `gosec` and `govulncheck` are clean, SSRF guidance exists, auth middleware is cautious, and security headers/rate limits are built in. Residual risk is mostly around behavioral correctness rather than classic source-to-sink flaws. |
| Test effectiveness                     |  8/10 | The repo has broad unit coverage across critical packages and includes race/fuzz targets. The main weakness is missing regression coverage for rare failure-path contracts. |
| Change safety & backward compatibility |  7/10 | API diff tooling and extensive tests help, but some public contracts were able to drift without failing the current suite. |
| Operability & observability            |  8/10 | Health, metrics, request logging, panic recovery, and scheduler recorder handling are all present and documented. |
| Clarity & developer experience         |  8/10 | README, architecture docs, and Makefile quality gates are strong. A few large packages now require more local context than they should. |
| Extensibility                          |  8/10 | The ports/adapters model makes new integrations straightforward. The main drag is that a handful of large behavioral packages are accumulating branching logic. |
| Overall                                |  8/10 | A solid production-oriented toolkit with good structure and tooling, but with a few correctness bugs in high-sensitivity runtime contracts. |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- `middleware/idempotency/idempotency.go:213` can continue to execute the downstream handler after `TryBegin` loses the reservation race when the subsequent `Get` cannot confirm the stored state. The implementation comment at `:249` explicitly falls back to fresh processing. That contradicts the documented safe-retry guarantee in [README.md](../../README.md) and [docs/cookbook.md](../../docs/cookbook.md), and it opens a duplicate-execution path in exactly the class of request this middleware exists to protect.

### Medium
- `ports/health.go:74` documents `HealthCheckConfig.Timeout` as bounding “a single liveness, readiness, or detailed health pass”, but `endpoints/health/health.go:96` runs `GetDetailedHealth` without wrapping the pass in a timeout context. A slow or blocked checker can therefore hang detailed health indefinitely even when a timeout is configured.
- `contrib/adapters/httpclient/client.go:146` retries replayable methods by resetting the request body on attempts after the first. If the first attempt returns a retryable response and the request body lacks `GetBody`, `resetBody` at `:148` returns `request body is not replayable`, after the client has already discarded and closed the original response body at `:166`. The caller loses the real upstream failure and gets a synthetic local error instead.

### Low
- `middleware/idempotency/idempotency.go`, `endpoints/health/checkers.go`, `contrib/migrator/migrator.go`, and several bootstrap helpers contain dense branching around timeouts, persistence states, and fallback behavior. The code is still readable, but these files are now the main extension risk in the repo because contract logic and transport/persistence handling live in the same functions.

## Hexagonal architecture verdict
What is clean:
- `ports` is dependency-light and stable.
- Core middleware/endpoints avoid hard dependencies on concrete router/database/logging stacks.
- Contrib adapters generally implement inward-facing contracts rather than leaking vendor types into core packages.

What leaks across boundaries:
- `contrib/bootstrap` includes convenience `OrExit` helpers that mix library code with process-lifecycle decisions.
- Some large middleware packages embed policy, persistence-state handling, and HTTP response shaping in the same functions, which makes the architectural boundary clean at the package level but less clean internally.

Verdict:
- The codebase is partially hexagonal to strongly hexagonal. It is not purely domain-driven, because this repo is a transport/runtime toolkit rather than a business domain app, but the dependency direction is mostly correct and the adapter story is real rather than ceremonial.

## Test verdict
What is covered well:
- Middleware and endpoint contracts, including auth, trace, timeout, rate limiting, panic recovery, migrator edge cases, and adapter integrations.
- Repository-standard quality gates are unusually strong for a utility toolkit: unit, race, fuzz, lint, vuln, and gosec targets all exist.

What is weak:
- Rare failure-path regressions where the implementation must preserve a documented guarantee under partial failure or retry pressure.
- Detailed health timeout behavior and outbound retry/body replay interaction were not explicitly locked down by tests.

Verdict:
- The tests are confidence-building, not superficial, but they still underrepresent a few high-value negative paths. The suite proves that the repo is disciplined; it does not yet prove every contract around retries, timeouts, and idempotent race resolution.

## Best next fixes
1. Make idempotency reservation collisions fail closed when the store cannot confirm the existing record, and add regression coverage for the duplicate-execution path.
2. Apply `HealthCheckConfig.Timeout` to `GetDetailedHealth` so detailed probes obey the documented contract under slow dependencies.
3. Preserve the original upstream response/error when retry logic encounters a non-replayable request body instead of returning a synthetic local failure.
4. Keep adding failure-path contract tests around the largest behavioral packages (`middleware/idempotency`, `endpoints/health`, `contrib/adapters/httpclient`, `contrib/migrator`).

## Optional follow-up
- Targeted remediation plan: implemented in `.audits/codebase_audit_v9/remediation_backlog.md`.
- Security-focused pass: worthwhile after these correctness fixes, especially around auth skip-header and outbound HTTP configuration misuse.
