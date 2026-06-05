# Auth Production Guide

Audience: teams configuring API-key, JWT, OIDC, Clerk, tenant, and role
authorization for api-toolkit services.

Auth should fail closed before handlers run. Applications still own identity
provider configuration, secret storage, tenant model, role model, and product
authorization policy.

## API Keys

| Area | Production setting |
| --- | --- |
| Storage | Store only hashes or peppered hashes. Raw API keys should be returned once at creation and never logged. |
| Prefix display | Display non-secret prefixes only. Prefixes are lookup hints, not credentials. |
| Scope checks | Bind scopes to route metadata and reject insufficient scopes before handlers run. |
| Revocation | Reject revoked keys and record bounded `last_used_at` metadata when useful. |
| Generated services | Set `API_KEY_PEPPER` explicitly in production. Treat bootstrap `API_KEY` as setup-only. |

## JWT, OIDC, and Clerk

| Area | Production setting |
| --- | --- |
| Issuer | Configure an exact trusted issuer; do not infer it from requests. |
| Audience | Require the intended API audience. |
| Algorithms | Allow only expected algorithms. Reject `none` and unexpected alg values. |
| JWKS URL | Treat JWKS and discovery URLs as trusted operator configuration. |
| JWK rotation | Cache keys with bounded refresh behavior and test new `kid` rollout. |
| Clock skew | Use a small explicit skew allowance and test expired, not-before, and issued-at boundaries. |
| Failure mode | Missing, malformed, expired, wrong-issuer, wrong-audience, unknown-`kid`, and unverifiable tokens fail closed with 401. |

Do not enable JWT, Clerk, OIDC, or dev-header skip/bypass headers outside local
development. Production startup should reject dangerous bypass configuration.

## Tenant and Role Authorization

Apply identity, tenant, and role checks before product handlers:

1. Authenticate the actor.
2. Extract tenant scope from a trusted token claim, API-key record, or app-owned
   session.
3. Compare all required tenant sources, such as path, header, and identity.
4. Require role or scope for the route.
5. Run idempotency after auth and tenant checks for unsafe writes.

Use `RequireAllSources` for routes that must prove `X-Tenant-ID`, route path,
and authenticated tenant scope match. A tenant mismatch should fail closed with
403 or a route-specific Problem Details response before side effects run.

## Testing

Add tests for:

- missing credentials,
- invalid API key hash,
- revoked API key,
- insufficient scope,
- wrong tenant in path/header/token,
- expired JWT,
- wrong issuer,
- wrong audience,
- unknown JWK `kid`,
- JWK rotation,
- clock skew boundaries,
- dev bypass disabled in production,
- route metadata requiring auth and tenant checks.

Keep raw credentials, bearer tokens, API keys, JWKs, session IDs, and provider
payloads out of logs, metrics, OpenAPI examples, Problem Details, and release
evidence.
