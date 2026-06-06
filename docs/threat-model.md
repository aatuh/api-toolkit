# Security Threat Model

Audience: maintainers, application teams, and security reviewers deciding
whether api-toolkit controls are sufficient for a conventional Go JSON/HTTP API
or generated service.

This document summarizes the security assumptions, protected assets, threat
surfaces, required mitigations, and verification evidence for the project. It is
the review map; setup details live in the linked production guides.

## Scope

In scope:

- root HTTP middleware and helpers,
- auth, tenant, idempotency, webhook, health, metrics, pprof, and OpenAPI
  guardrails,
- supported contrib adapters when they are wired by generated services,
- generated `saas-api`, `saas-api-full`, `dev-api`, and `saas-web` scaffolds,
- release evidence that proves these controls are tested or documented.

Out of scope:

- product-specific authorization policy,
- customer data classification and retention policy,
- provider account hardening outside generated or adapter-owned boundaries,
- deployment network policy after starter manifests are copied into an
  application repository,
- live incident response for downstream services.

Applications own those out-of-scope decisions even when they use toolkit
defaults.

## Baseline Assumptions

- Public API routes are reachable from untrusted networks.
- Clients can send malformed, oversized, duplicated, or replayed requests.
- Bearer tokens, API keys, tenant headers, webhook signatures, and idempotency
  keys can be missing, forged, stale, or leaked by callers.
- Proxies can add or remove headers unless trusted-proxy configuration is
  explicit.
- Metrics, logs, traces, Problem Details, OpenAPI examples, release evidence,
  and audit metadata can become data-leak channels if they include raw secrets,
  tenant IDs, request bodies, provider payloads, or unbounded user input.
- Generated services are app-owned after generation; toolkit support covers the
  template, defaults, and evidence, not downstream modifications.

## Protected Assets

| Asset | Primary risk | Required protection |
| --- | --- | --- |
| API keys, admin keys, session IDs, CSRF tokens, webhook secrets, provider tokens | Credential disclosure or replay | Return raw secrets only at creation when necessary, store hashes or encrypted values, and keep raw values out of logs, metrics, Problem Details, OpenAPI examples, release evidence, and audit metadata. |
| JWT/JWK and OIDC trust configuration | Token confusion or verifier bypass | Treat issuer, audience, algorithms, JWKS URL, and discovery URL as trusted operator configuration; reject unknown issuers, audiences, algorithms, and `kid` values. |
| Tenant and actor context | Cross-tenant access or wrong-actor side effects | Authenticate first, derive tenant from trusted identity or API-key records, then compare every required path/header/token source before handlers run. |
| Unsafe write side effects | Duplicate writes, conflicting retries, or replay across tenants | Require idempotency keys where route contracts need them, hash request identity, store replay state durably for multi-instance services, and scope storage keys by tenant and actor. |
| Admin endpoints, pprof, metrics, detailed health | Operational data disclosure or active profiling exposure | Keep detailed health, metrics, and pprof on admin/internal routes with explicit auth or network policy; public probes stay minimal. |
| Webhook payloads and delivery state | Forged callbacks, replay, SSRF, secret leakage, or provider payload disclosure | Verify signatures before trust, cap bodies, enforce replay windows, suppress duplicate deliveries, use trusted outbound transports, and redact delivery metadata. |
| Generated scaffolds and deployment starters | Unsafe defaults copied into production | Treat generated code and deployment assets as app-owned templates; require production env validation, secret replacement, network policy review, and service-owned tests. |

## Threat Surface Matrix

| Surface | Threats | Required mitigations | Verification evidence |
| --- | --- | --- | --- |
| Auth entrypoints | Missing credentials, invalid credentials, dev bypasses enabled in production, auth middleware mounted after side effects. | Apply auth before product handlers; disable dangerous bypasses in production; fail closed with 401 or route-specific Problem Details; keep raw credentials out of observability surfaces. | [security posture](security.md), [auth production guide](auth.md), [safe defaults](safe-defaults.md), [negative-path matrix](negative-path-test-matrix.tsv). |
| JWT/JWK bearer validation | Wrong issuer, wrong audience, `none` or unexpected algorithms, stale JWKS cache, unknown `kid`, expired/not-yet-valid tokens, clock-skew bugs. | Configure exact issuer and audience; allow only expected algorithms; treat JWKS/discovery URLs as trusted operator config; bound refresh behavior; test rotation and time boundaries. | [auth production guide](auth.md), [core readiness matrix](core-readiness.md), [context and cancellation](context-cancellation.md). |
| API keys | Raw key storage, key logging, prefix treated as a credential, revoked key accepted, missing scope check, bootstrap key used permanently. | Store hashes or peppered hashes; expose only non-secret prefixes; return raw keys once; reject revoked or insufficient-scope keys; replace bootstrap setup keys with scoped generated keys. | [auth production guide](auth.md), [security posture](security.md), [full-service scaffold](full-service-scaffold.md). |
| Tenant context and authorization | Tenant header spoofing, path/header/token mismatch, tenant context missing before idempotency, app-owned role checks skipped. | Authenticate first; derive tenant scope from trusted identity or key records; require all configured tenant sources to agree; run tenant checks before idempotency and side effects; keep role policy app-owned and tested. | [auth production guide](auth.md), [security posture](security.md), [OpenAPI workflow](openapi-workflow.md), [negative-path matrix](negative-path-test-matrix.tsv). |
| Idempotency replay | Duplicate side effects after retry, same key with different payload replayed, cross-tenant replay, stale reservation release, replay body leakage. | Require `Idempotency-Key` on unsafe writes that need retry safety; hash method, route, tenant, actor, and request body metadata; use token-aware reservation release; use durable shared storage for multi-instance services; exclude streaming responses. | [idempotency guide](idempotency.md), [middleware safety](middleware-safety.md), [input-size threat review](input-size-threat-review.md). |
| Admin endpoints | Detailed health, admin actions, or dependency state exposed through public routes. | Split public liveness/readiness from admin detail; mount detailed health through explicit admin wrappers or private listeners; fail closed when required admin auth is missing. | [operations guide](operations.md), [security posture](security.md), [reference service](reference-service.md). |
| Pprof and metrics | Internet-exposed profiling, high-cardinality or sensitive labels, tenant IDs or keys in metrics, operational data leaked to clients. | Mount pprof only through `pprof.RegisterAdminRoutes` or equivalent admin wrappers; expose metrics on admin/internal routes; use route patterns and bounded labels; keep secrets, raw paths, tenant IDs, request bodies, and provider payloads out of labels. | [operations guide](operations.md), [metrics guide](metrics.md), [observability guide](observability.md). |
| Webhooks and provider callbacks | Forged signatures, replayed callbacks, oversized payloads, duplicate deliveries, SSRF through receiver URLs, signing-secret disclosure, provider payload leakage. | Verify signatures before trust; enforce body caps and replay windows; suppress duplicates in app-owned stores; use trusted outbound transports; encrypt generated endpoint secrets when persisted; redact delivery, audit, Problem Details, metrics, and OpenAPI surfaces. | [webhook recipe](cookbook.md#webhook-receiver-with-signature-verification), [security posture](security.md), [resource lifecycle](resource-lifecycle.md), [full-service scaffold](full-service-scaffold.md). |
| Generated scaffolds | Local defaults promoted to production, generated dev auth used in production, secrets committed, scaffold regenerated over app-owned changes, unsupported provider assumptions. | Treat generated code as app-owned; validate production env; disable dev bypasses; replace placeholder secrets; compare regeneration diffs manually; run generated tests and deployment checks owned by the service. | [scaffold support matrix](scaffold-support.md), [full-service scaffold](full-service-scaffold.md), [getting started](getting-started.md), [reference service README](../examples/reference-saas-api/README.md). |

## Review Triggers

Run a threat-model review before merging any change that:

- adds or changes authentication, tenant, role, session, webhook, idempotency,
  admin, metrics, pprof, or generated scaffold behavior,
- accepts a new header, query parameter, path parameter, cookie, webhook body,
  provider payload, object key, or environment variable as trusted input,
- changes request or response buffering limits,
- changes logs, metrics labels, traces, audit metadata, OpenAPI examples, or
  release evidence fields,
- changes generated service production defaults, dangerous bypasses, secret
  handling, deployment starter assets, or integration checks.

## Minimum Verification

For a security-sensitive change, keep verification at the right boundary:

- unit or contract tests for parser, verifier, middleware, or helper behavior,
- negative-path tests for malformed input, missing credentials, wrong tenant,
  expired JWT, invalid signature, missing idempotency key, and oversized input,
- generated-service checks when scaffold wiring or templates change,
- docs contracts when a security control depends on documented operator steps,
- release evidence review when the claim is part of publication readiness.

Use [testing policy](testing.md), [release review](release-review.md), and
[release runbook](release-runbook.md) for the exact gate expectations.
