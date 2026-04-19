## Executive summary
This repository remains a strong Go API toolkit with a credible ports-and-adapters structure, broad package-level test coverage, and unusually good built-in quality gates. During this audit, `make test`, `make test-race`, `make lint`, `make vuln`, `make gosec`, `make api-check`, and `make fuzz` all passed, which raises confidence that the remaining problems are concentrated in boundary contracts rather than systemic instability.

The core/contrib split is still the codebase’s strongest architectural choice. Stable interfaces live in `ports`, most framework and vendor details stay in contrib, and the public middleware/endpoints packages are small enough to reason about. The repo is also disciplined about explicit contracts in the README and release notes, which made it possible to compare runtime behavior against documented expectations.

The highest-severity defect is in the migration runner’s embedded filesystem support. `Options.EmbeddedFSs` is documented as expecting a `migrations` directory (`contrib/migrator/migrator.go:41-44`), and the README shows `//go:embed migrations/*.sql` as the supported usage (`README.md:241-248`), but `loadMigrations` currently reads only the embedded filesystem root and skips directories entirely (`contrib/migrator/migrator.go:572-605`). In practice, the documented embed-based wiring does not load any migrations.

Two smaller but still meaningful correctness issues remain in public wiring helpers. `endpoints/health.CustomChecker` dereferences `checkFunc` unconditionally (`endpoints/health/checkers.go:146-185`), so a nil checker function turns an operational health endpoint into a panic path instead of returning a controlled unhealthy or unknown result. In contrib bootstrap, both `ProfileStrictAPI` and `ProfileDev` still construct `querylimits.New(cfg.queryLimits)` even when the feature is later disabled by `queryLimitsMiddleware` (`contrib/bootstrap/profile.go:213-216`, `contrib/bootstrap/profile.go:313-316`, `contrib/bootstrap/profile.go:377-381`), so disabled query guardrails can still break profile construction when their options are invalid.

Overall, this is a good production-oriented codebase. The remaining work is high-leverage: fix the documented embedded-migrations contract, harden one health-check constructor against panic, and make bootstrap feature toggles behave like true toggles.

## Scorecard
| Dimension                              | Score | Notes |
|----------------------------------------|------:|-------|
| Architecture & boundaries              |  8/10 | Core and contrib are cleanly separated, but some bootstrap helper behavior still leaks disabled-feature validation into runtime wiring. |
| SOLID / cohesion / coupling            |  8/10 | Packages are focused and mostly composable; a few constructor paths still mix configuration normalization with disabled features. |
| Correctness & robustness               |  7/10 | Built-in gates are strong, but embedded migration loading and nil custom health functions both break caller expectations in real wiring paths. |
| Security                               |  8/10 | Static security tooling is clean and SSRF/auth hardening is thoughtful; no new direct security flaw was found in this pass. |
| Test effectiveness                     |  8/10 | Test breadth is good, but the documented `embed.FS` migrator path and nil custom health checker path were not covered. |
| Change safety & backward compatibility |  7/10 | API compatibility tooling passes, but the embedded migration behavior currently diverges from the documented usage shown to downstream consumers. |
| Operability & observability            |  8/10 | Health, logging, and migration observability are generally strong; the custom checker panic path weakens operational safety under misconfiguration. |
| Clarity & developer experience         |  8/10 | Naming and docs are strong overall; the current migrator docs/runtime mismatch is the largest DX regression. |
| Extensibility                          |  8/10 | The toolkit is easy to extend, but feature toggles in bootstrap should not force validation of subsystems callers explicitly disabled. |
| Overall                                |  8/10 | Strong foundation with three concrete boundary defects worth fixing immediately. |

Confidence: high

## Findings by severity
### Critical
- None.

### High
- Embedded migration loading is broken for the repo’s documented `embed.FS` usage. `Options.EmbeddedFSs` says each filesystem is expected to contain a `migrations` directory (`contrib/migrator/migrator.go:41-44`), and the README instructs callers to embed `migrations/*.sql` and pass that filesystem directly (`README.md:241-248`). But `loadMigrations` reads `fs.ReadDir(root, ".")` and skips directories (`contrib/migrator/migrator.go:572-605`), so embedded files under `migrations/` are never discovered. This is a real runtime contract failure, not a documentation nit.

### Medium
- `CustomChecker` panics when wiring passes a nil check function. Both constructors accept `checkFunc` without validation (`endpoints/health/checkers.go:153-168`), and `Check` calls `c.checkFunc(checkCtx)` unconditionally (`endpoints/health/checkers.go:176-185`). A misconfigured health checker should fail closed with a result, not take the endpoint down via panic.
- `ProfileStrictAPI` and `ProfileDev` still validate query-limit options even when query limits are disabled. Both constructors eagerly call `querylimits.New(cfg.queryLimits)` (`contrib/bootstrap/profile.go:213-216`, `contrib/bootstrap/profile.go:313-316`), while the actual middleware application later no-ops when disabled (`contrib/bootstrap/profile.go:377-381`). This makes `WithQueryLimitsDisabled()` incomplete: disabled guardrails can still block profile construction on stale or invalid options.

### Low
- None.

## Hexagonal architecture verdict
What is clean:
- `ports` remains a meaningful inward contract surface for HTTP, health, database, logging, validation, and integrations.
- Core packages generally stay free of third-party framework types, with contrib carrying most adapter-specific wiring.
- Public helpers are usually small and composable, which keeps change scope manageable.

What leaks across boundaries:
- Contrib bootstrap still lets a disabled middleware feature influence profile construction, which is a configuration/wiring boundary leak rather than a domain-layer leak.
- The migration runner’s documented embedded-filesystem contract is not preserved by the loader implementation, which is a helper/runtime contract leak between docs and adapter behavior.

Verdict:
- Partially hexagonal. The codebase is materially ports-and-adapters oriented, but a few public helper constructors still behave more like pragmatic layered utilities than strict boundary-preserving adapters.

## Test verdict
What is covered well:
- `make test`, `make test-race`, `make lint`, `make vuln`, `make gosec`, `make api-check`, and `make fuzz` all passed during this audit.
- Core middleware, health handlers, scheduler behavior, HTTP client retry logic, and many contrib adapters already have meaningful regression coverage.

What is weak:
- No test currently protects the documented `embed.FS` migration loading path.
- No test currently proves that nil custom health check functions fail safely.
- No test currently locks in that disabled bootstrap query limits should skip query-limit construction entirely.

Verdict:
- Tests are confidence-building overall, but three public wiring paths still lacked the exact regression tests needed to keep contract drift from slipping through.

## Best next fixes
1. Restore embedded migration loading for `EmbeddedFSs` and add a regression test that uses the README-style `migrations/*.sql` layout.
2. Make `CustomChecker` return a safe `unknown` result when the check function is missing, and default non-positive custom timeouts back to the standard timeout.
3. Stop contrib bootstrap from constructing query-limit middleware when callers explicitly disable query guardrails, and add regression tests for both strict and dev profiles.

## Optional follow-up
- A focused second pass on helper-constructor contracts would likely uncover a few more docs/runtime mismatches in bootstrap-style convenience APIs.
- A package-by-package contrib audit could further pressure-test misconfiguration handling at external boundaries.
