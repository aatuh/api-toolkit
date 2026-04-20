# Versioning and Stability

This project follows semantic versioning for the core module
`github.com/aatuh/api-toolkit/v2`. From v1 onward, we treat the packages listed
below as stable: any breaking change requires a major version bump.

## Stable API surface (core module)

All exported identifiers in these packages are considered stable:

- `github.com/aatuh/api-toolkit/v2/authorization`
- `github.com/aatuh/api-toolkit/v2/email`
- `github.com/aatuh/api-toolkit/v2/endpoints/docs`
- `github.com/aatuh/api-toolkit/v2/endpoints/health`
- `github.com/aatuh/api-toolkit/v2/endpoints/list`
- `github.com/aatuh/api-toolkit/v2/endpoints/pprof`
- `github.com/aatuh/api-toolkit/v2/endpoints/version`
- `github.com/aatuh/api-toolkit/v2/fielderrors`
- `github.com/aatuh/api-toolkit/v2/httpx`
- `github.com/aatuh/api-toolkit/v2/httpx/identity`
- `github.com/aatuh/api-toolkit/v2/httpx/recover`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/authz`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/jwt`
- `github.com/aatuh/api-toolkit/v2/middleware/auth/tenant`
- `github.com/aatuh/api-toolkit/v2/middleware/idempotency`
- `github.com/aatuh/api-toolkit/v2/middleware/json`
- `github.com/aatuh/api-toolkit/v2/middleware/maxbody`
- `github.com/aatuh/api-toolkit/v2/middleware/querylimits`
- `github.com/aatuh/api-toolkit/v2/middleware/ratelimit`
- `github.com/aatuh/api-toolkit/v2/middleware/secure`
- `github.com/aatuh/api-toolkit/v2/middleware/timeout`
- `github.com/aatuh/api-toolkit/v2/middleware/trace`
- `github.com/aatuh/api-toolkit/v2/ports`
- `github.com/aatuh/api-toolkit/v2/response_writer`
- `github.com/aatuh/api-toolkit/v2/scheduler`
- `github.com/aatuh/api-toolkit/v2/scheduler/migrations`
- `github.com/aatuh/api-toolkit/v2/securityprofile`
- `github.com/aatuh/api-toolkit/v2/specs`
- `github.com/aatuh/api-toolkit/v2/swagstub`

## Experimental or unstable surfaces

- The `contrib` module is experimental and may change in minor releases.
- `github.com/aatuh/api-toolkit/v2/middleware/auth/shared` is an implementation-sharing package for auth middleware and is not part of the stable compatibility promise.
- `github.com/aatuh/api-toolkit/v2/response_writer` is legacy, but it remains part of the stable compatibility promise until explicitly removed in a major release.
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
