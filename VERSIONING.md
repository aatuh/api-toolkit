# Versioning and Stability

This project follows semantic versioning for the core module
`github.com/aatuh/api-toolkit`. From v1 onward, we treat the packages listed
below as stable: any breaking change requires a major version bump.

## Stable API surface (core module)

All exported identifiers in these packages are considered stable:

- `github.com/aatuh/api-toolkit/authorization`
- `github.com/aatuh/api-toolkit/email`
- `github.com/aatuh/api-toolkit/endpoints/docs`
- `github.com/aatuh/api-toolkit/endpoints/health`
- `github.com/aatuh/api-toolkit/endpoints/list`
- `github.com/aatuh/api-toolkit/endpoints/pprof`
- `github.com/aatuh/api-toolkit/endpoints/version`
- `github.com/aatuh/api-toolkit/fielderrors`
- `github.com/aatuh/api-toolkit/httpx`
- `github.com/aatuh/api-toolkit/httpx/identity`
- `github.com/aatuh/api-toolkit/httpx/recover`
- `github.com/aatuh/api-toolkit/middleware/auth/authz`
- `github.com/aatuh/api-toolkit/middleware/auth/jwt`
- `github.com/aatuh/api-toolkit/middleware/auth/tenant`
- `github.com/aatuh/api-toolkit/middleware/idempotency`
- `github.com/aatuh/api-toolkit/middleware/json`
- `github.com/aatuh/api-toolkit/middleware/maxbody`
- `github.com/aatuh/api-toolkit/middleware/querylimits`
- `github.com/aatuh/api-toolkit/middleware/ratelimit`
- `github.com/aatuh/api-toolkit/middleware/secure`
- `github.com/aatuh/api-toolkit/middleware/timeout`
- `github.com/aatuh/api-toolkit/middleware/trace`
- `github.com/aatuh/api-toolkit/ports`
- `github.com/aatuh/api-toolkit/response_writer`
- `github.com/aatuh/api-toolkit/scheduler`
- `github.com/aatuh/api-toolkit/scheduler/migrations`
- `github.com/aatuh/api-toolkit/securityprofile`
- `github.com/aatuh/api-toolkit/specs`
- `github.com/aatuh/api-toolkit/swagstub`

## Experimental or unstable surfaces

- The `contrib` module is experimental and may change in minor releases.
- `github.com/aatuh/api-toolkit/middleware/auth/shared` is an implementation-sharing package for auth middleware and is not part of the stable compatibility promise.
- Examples, docs, and tooling are not API commitments.
- Any package explicitly documented as experimental is unstable.

## Deprecation policy

- Use `// Deprecated:` Go doc comments with a replacement when possible.
- Deprecated APIs remain for at least one minor release unless a security fix
  requires removal.

## API compatibility checks

CI runs `scripts/apicheck.sh` to detect incompatible changes in the stable
packages. Breaking changes must coincide with a major version bump.

## Behavioral upgrade guidance

- API compatibility checks protect exported identifiers in the stable surface,
  not every runtime contract or operator-facing default.
- Review [docs/release-notes.md](/home/aatu/projects/saas/api-toolkit/docs/release-notes.md)
  on every upgrade for behavior changes around health endpoint exposure,
  scheduler observability, transaction cleanup, and `contrib` migration state
  handling.
- New helper packages added to support internal refactors should be treated as
  unstable unless they are listed in the stable API surface above.

## Related policies

- Panic usage is governed by `PANIC_POLICY.md`.
