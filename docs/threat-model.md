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

## Workflow Trust Boundaries

Repository workflows are a supply-chain boundary. The checked-in policy is
enforced by `make actions-audit` and `make workflow-security-check`; GitHub
branch rules, environment protections, and required-check enforcement remain
external configuration verified through EXT-002.

| Workflow class | Trusted inputs and permissions | Required boundary |
| --- | --- | --- |
| Pull-request CI, CodeQL, integration, and dependency review | Untrusted fork or contributor code; workflow-level `contents: read`. CodeQL may write security events only. | No `pull_request_target`, `workflow_run`, repository-write permission, release token, provider secret, artifact download, or shared Actions cache. Pull-request title/body fields are passed only as data to bounded validators, never interpolated directly into shell. |
| Scheduled provider sandbox | Default-branch workflow plus protected sandbox environment. | Provider secrets exist only in the `nightly.yml` sandbox job; outputs are sanitized evidence with explicit retention. The workflow has no pull-request trigger. |
| Release | Protected push tag and scoped release-preflight job permissions. | `contents: write`, attestation, and OIDC permissions are confined to release preflight after compatibility and contract jobs. Tags are validated before publication; release steps cannot use `continue-on-error`. |
| Scorecard and CodeQL reports | Default-branch/scheduled events with narrowly scoped reporting permissions. | Immutable SHA-pinned actions, no checkout credential persistence where no write is required, and bounded artifact retention. |

The static gate rejects mutable action references through the companion actions
audit, broad workflow permissions, unsafe privileged triggers, secrets in
pull-request workflows, direct shell interpolation of event data, artifact
downloads, shared Actions caches, unbounded artifact retention, and unsafe
release failure handling. It reports only workflow paths and rule names; it
does not read, print, or persist secret values.

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

## Architecture Risk Register

Each row has one accountable owner and points to the boundary-level check that
must change with its mitigation. Package-level changes additionally require the
[package security review](security-review.md), whose public-package ownership
records are maintained in [the package owners manifest](package-owners.tsv).

| Asset | Actor / entry point | Trust boundary and abuse case | Existing mitigation and required test | Residual risk / owner |
| --- | --- | --- | --- | --- |
| HTTP parser and request body | Internet client; request line, headers, query, path, JSON/form/multipart body | Untrusted bytes become typed input. Abuse includes malformed encoding, ambiguous parameters, oversized or decompression-amplified bodies, parser differential, or context cancellation ignored by downstream work. | Bounded parsing and body policies, explicit binding/validation, duplicate-source policy, and context-aware handlers. Run parser and negative-path tests; see [input-size threat review](input-size-threat-review.md) and [middleware safety](middleware-safety.md). | Application field limits and reverse-proxy limits must agree with toolkit limits. Core HTTP maintainers. |
| Response writer, streaming response, and Problem Details | Handler/plugin or client-controlled error value; response body, headers, trailers, stream | Application errors cross to public HTTP output. Abuse includes a partial success after error, headers/body written twice, unbounded stream retention, or sensitive cause, tenant, credential, and provider data exposed in Problem Details. | Single-write response controls, bounded streaming policy, typed Problem Details, and redaction rules. Run response-writer, streaming, and Problem Details negative tests; see [error handling](errors.md) and [OpenAPI workflow](openapi-workflow.md). | Application-owned errors and streaming protocols can still disclose data if mapped unsafely. Core HTTP maintainers. |
| Proxy identity and client address | Reverse proxy, forwarded headers, trusted-proxy configuration | Forwarded identity crosses from network infrastructure to authorization, audit, and rate-limit decisions. An attacker can spoof client IP/proto/host or use an untrusted proxy as a confused deputy. | Explicit trusted-proxy configuration, bounded forwarding-header parsing, and no implicit trust of public headers. Run proxy identity and rate-limit boundary tests; see [operations guide](operations.md). | Proxy topology and network ACLs are operator-owned. Operations maintainers. |
| Authentication, authorization, and tenant context | Caller credentials, route/header identifiers, token claims, API-key records | Identity claims cross into actor, role, and tenant scope. Abuse includes bypasses, token confusion, route order mistakes, wrong-owner access, and mismatch between path, header, and token tenant identifiers. | Authenticate before side effects; exact JWT/JWK validation; hashed API keys; derive tenant from trusted identity; deny mismatches before handlers. Run auth, authorization, and cross-tenant negative tests; see [auth production guide](auth.md) and [security posture](security.md). | Product role policy and identity-provider configuration are application-owned. Security and core maintainers. |
| Idempotency store and unsafe writes | Retrying client; idempotency key, method, route, tenant, actor, request identity | A caller-supplied retry key crosses into durable replay state. Abuse includes cross-tenant replay, changed-payload replay, duplicate side effects, stale reservations, and cache exhaustion. | Scope and hash replay identity, require compatible request data, reserve/release safely, use durable shared storage for multi-instance services, and exclude streams. Run idempotency store and race/negative tests; see [idempotency guide](idempotency.md). | Retention duration and downstream side-effect semantics are application-owned. Core and adapter maintainers. |
| Rate limiting and admission control | Internet client/proxy identity; per-route limit configuration | Untrusted identity and request volume cross into shared counters. Abuse includes spoofed keys, tenant starvation, counter exhaustion, and limit bypass through alternate routes or streams. | Use trusted identity only, cap keys and labels, scope limits deliberately, bound memory, and document stream handling. Run rate-limit keying and exhaustion tests; see [middleware safety](middleware-safety.md). | Capacity budgets and upstream DDoS protections require operator controls. Core HTTP maintainers. |
| Webhook verification and delivery | Provider callback, signing header, replay metadata, outbound endpoint URL | Provider-controlled payloads cross into side effects and delivery state. Abuse includes forged or replayed events, body exhaustion, SSRF, duplicate delivery, and leaked signing data. | Verify before trust, bound bodies/windows, deduplicate, use trusted outbound transports, and redact records. Run webhook verification/delivery contracts; see [webhook recipe](cookbook.md#webhook-receiver-with-signature-verification). | Provider policy, egress firewall, and endpoint ownership remain application-owned. Adapter maintainers. |
| Health, metrics, pprof, and operator-only endpoints | Internet client or operator; probe/admin route and observability labels | Operational state crosses to an endpoint or telemetry backend. Abuse includes public pprof, detailed health disclosure, high-cardinality exhaustion, and secret/tenant/body leakage. | Separate minimal public probes from authenticated/private admin routes; mount pprof explicitly; use bounded, redacted labels. Run route-exposure and observability checks; see [operations guide](operations.md) and [observability guide](observability.md). | Network placement, telemetry retention, and admin credentials are operator-owned. Operations maintainers. |
| PostgreSQL, Redis, and provider adapters | Remote database/provider; operator DSN/URL/configuration, webhook, or outbound delivery | Credentials, endpoint URLs, retry/timeout behaviour, and provider payloads cross adapter boundaries. Abuse includes SSRF, stale/replayed records, data leakage, or unavailable dependencies causing unsafe retry. | Supported-adapter contracts, bounded contexts, provider/webhook verification, real Postgres/Redis checks, and sanitized provider-fixture evidence. Run `make test-postgres`, `make test-redis`, and relevant generated integration checks. | Provider account policy, network egress, encryption, backup, and availability design remain application/operator-owned. Adapter owners. |
| CLI output tree and generated project | Local operator or malicious local filesystem; `new service --dir` and project commands | CLI arguments cross into a filesystem that may contain traversal, symlink, hard-link, pipe, device, socket, or user-owned paths. An overwrite could destroy app code or escape the approved root. | Approved-root validation, `os.Root` staging/publish, restrictive permissions, explicit check/fail/overwrite modes, and generated ownership manifests. Run `make cli-security-check`, `make cli-determinism-check`, and `make cli-offline-check`. | A trusted local account can still replace files between operations or choose a dangerous destination intentionally. CLI maintainers. |
| Generated secrets and deployment assets | Application team copying `.env.example`, Kubernetes starter files, or provider configuration | Placeholder values cross into deployment configuration. A team may commit a live secret, expose an admin route, or apply starter network policy without environment review. | Templates contain placeholders, generated ignores exclude local env files, startup validation and admin split are documented. Run generated-service tests and application deployment review. | Secret-manager choice, cluster policy, live credentials, and public ingress are application-owned. Application team. |
| Release assets and workflow identity | Tag push, GitHub workflow, downloaded release asset | Build inputs cross into GitHub OIDC, SBOM signing, manifest checksum, and provenance verification. Abuse includes wrong tag/repository/workflow identity, modified asset, missing SBOM/attestation, or stale certificate. | SHA-pinned workflows, scoped permissions, SBOM signatures, artifact verifier, and negative contracts. Run `make release-authenticity-check` and publication-mode artifact verification before publishing. | Protected tags, GitHub environment controls, and actual uploaded asset verification are external controls. Release maintainers. |
| Go module supply chain | Module proxy/cache, dependency update, generated `go.mod`/`go.sum` | Immutable version/checksum data crosses from module proxy into root, contrib, CLI, and generated projects. Abuse includes mutable template state, unexpected dependency upgrade, compromised upstream, or missing checksum evidence. | Embedded CLI templates, reviewed default manifest/checksums, dependency review, license/vulnerability policies, and offline generation check. Run `make cli-offline-check`, dependency review, and release evidence. | Upstream compromise and proxy availability cannot be eliminated locally; pin, review, and rotate as needed. Dependency maintainers. |

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

For every affected package, record the result in the [package security
review](security-review.md). The review is package-scoped so a safe helper does
not hide an unreviewed adapter, generated route, or public package change.

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
