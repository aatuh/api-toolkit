# Troubleshooting

Audience: application developers and maintainers diagnosing common api-toolkit
adoption, upgrade, and local-check failures.

Use this guide before opening an issue. If the symptom involves a security
surface, also read `docs/security.md`, `docs/safe-defaults.md`, and
`docs/middleware-safety.md`.

## Common Issues

| Symptom | Likely cause | Fix | Verify |
| --- | --- | --- | --- |
| Go version mismatch | Root and contrib target Go 1.25.x, but the local toolchain is older or `GOTOOLCHAIN` downloads a different version. | Install Go 1.25.x or use the repository CI image. Run local repository gates with `GOTOOLCHAIN=local`. | `go version` and `GOTOOLCHAIN=local make docs-check`. |
| Contrib instability surprise | A package is `experimental`, `wrapper-only`, `tooling`, or `generated`, not stable core. | Check `docs/package-classification.tsv` and `docs/package-classification.md` before standardizing on a contrib package. | Confirm the package tier badge matches your risk tolerance. |
| Timeout buffering breaks streaming | `middleware/timeout` hard timeout, idempotency response capture, or OpenAPI response validation is wrapping a streaming, SSE, websocket, large-download, or optional-writer route. | Use route-specific opt-outs, `securityprofile.StreamingRouteOverride`, and `openapi.ResponseValidationOptions.ShouldValidate`. | Run route tests that assert flushing, hijacking, streaming, or large downloads still work. |
| Missing health checks | Liveness/readiness handlers were mounted without registered checkers or detailed health was exposed as a public route. | Register explicit checkers, keep liveness process-only, keep readiness dependency-aware, and mount detailed health on an admin/internal route. | Curl public probes and admin detailed health separately. |
| Idempotency storage confusion | Local memory storage was used in a multi-instance service, unsafe writes did not require `Idempotency-Key`, or storage keys were not tenant-scoped. | Use durable storage such as Redis for multi-instance services, set `Options.RequireKey`, and use tenant-scoped hashed storage keys after auth and tenant middleware. | Replay an unsafe write with the same key and with a mismatched request hash. |
| Auth config fails closed | JWT/OIDC/Clerk issuer, audience, JWKS URL, algorithm, or trusted-proxy dev bypass settings are missing or invalid. | Treat provider configuration as trusted operator config. Do not enable dev bypasses in production. | Test missing token, wrong audience, expired token, and tenant mismatch cases. |
| Strict JSON rejects requests | `middleware/json` is applied to routes that accept multipart, binary, webhook raw bodies, or body-less public probes. | Apply JSON middleware to JSON route groups only. Exclude multipart, binary, webhook raw-body, health, and docs routes. | Send each route's intended content type. |
| OpenAPI response validation fails on valid streams | Response validation buffers finite responses and is not safe for non-finite or optional-writer routes. | Keep request validation enabled and route-filter response validation with `openapi.ResponseValidationOptions.ShouldValidate`. | Add a streaming or large-download regression test. |
| Dependency boundary check fails | A stable root package imported contrib, a provider SDK, database driver, router adapter, generated app code, or example code. | Move the dependency into contrib, generated app code, or app-owned code. | `GOTOOLCHAIN=local make dependency-boundary-check`. |
| Generated scaffold changes overwrite product code | Generated services are app-owned code and regeneration was applied directly over edited service files. | Generate into a temporary directory, diff, and port intentional infrastructure changes manually. | Review git diff before running tests and OpenAPI contract checks. |

## Local Commands

Use the narrowest command that matches the failure:

```sh
GOTOOLCHAIN=local make docs-check
GOTOOLCHAIN=local make dependency-boundary-check
GOTOOLCHAIN=local make timeout-determinism-check
GOTOOLCHAIN=local make fast-check
```

For generated services, run the generated service's own test, openapi-check,
contracts-lint, and contracts-diff targets from inside that generated service
directory.

## When to Escalate

Open an issue when:

- the failure reproduces on a clean checkout,
- the package tier and docs say the behavior should work,
- the issue includes the exact command, Go version, package version, and failing
  request or test case,
- no secret, token, raw provider payload, tenant-controlled object key, or
  personal data is included in the report.
