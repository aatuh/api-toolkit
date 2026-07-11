# Core Readiness Matrix

Audience: API consumers, maintainers, and release reviewers deciding whether a
stable root package is ready for production standardization.

This matrix covers the v3 stable and compatibility-only root packages listed in
`VERSIONING.md`, `scripts/apicheck.sh`, and `docs/package-classification.tsv`.
It is a production review aid, not a replacement for package docs or release
evidence.

Status vocabulary:

- Docs: package docs plus any public guide linked from `docs/README.md`.
- Examples: compile-checked `Example...` coverage under `go test`.
- Tests: direct package tests listed in `docs/package-classification.tsv`.
- Fuzz: current fuzz posture or the package-specific fuzz trigger before
  expanding behavior.
- Benchmark: current benchmark baseline or reason a benchmark is not the first
  readiness signal.
- Compatibility: v3 SemVer gate, compatibility-only caveat, or release-note
  requirement.
- Security review: input, auth, tenant, timeout, replay, observability, or
  operator boundary to re-check before production rollout.
- Production caveat: what the application still owns.

## Stable Core Completeness Matrix

| Package | Docs | Examples | Tests | Fuzz | Benchmark | Compatibility | Security review | Production caveat |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `github.com/aatuh/api-toolkit/v4/apiclient` | Package docs | Compile checked | Direct tests | Add before broad parser changes | Not hot path | Stable API gate | Avoid leaking auth headers in errors | App owns transport auth and retries. |
| `github.com/aatuh/api-toolkit/v4/apitest` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Test helpers must not normalize unsafe responses away | App owns meaningful assertions. |
| `github.com/aatuh/api-toolkit/v4/authorization` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Fail closed on missing identity or scope | App owns policy model. |
| `github.com/aatuh/api-toolkit/v4/binding` | Package docs | Compile checked | Direct tests | `FuzzDecodeJSONAndQuery` smoke | Baseline exists | Stable API gate | Malformed and oversized input | App owns validation rules. |
| `github.com/aatuh/api-toolkit/v4/compat/billing` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Compatibility-only | Provider secrets and customer data | Use only for v3 migration compatibility. |
| `github.com/aatuh/api-toolkit/v4/contracttest` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Contract tests must fail closed on drift | App owns service-specific contracts. |
| `github.com/aatuh/api-toolkit/v4/email` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Email addresses and delivery metadata | App owns provider validation and consent. |
| `github.com/aatuh/api-toolkit/v4/endpoints/docs` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Public docs exposure and CDN opt-in | App owns route access policy. |
| `github.com/aatuh/api-toolkit/v4/endpoints/health` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Detailed health is operator-only | App owns dependency checks. |
| `github.com/aatuh/api-toolkit/v4/endpoints/list` | Package docs | Compile checked | Direct tests | Add before query contract expansion | Not hot path | Stable API gate | Pagination and filter bounds | App owns collection semantics. |
| `github.com/aatuh/api-toolkit/v4/endpoints/pprof` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | pprof must be admin/internal only | App owns network and auth wrapper. |
| `github.com/aatuh/api-toolkit/v4/endpoints/version` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Build metadata exposure | App owns what version data is public. |
| `github.com/aatuh/api-toolkit/v4/fielderrors` | Package docs | Compile checked | Direct tests | Add before parser/code mapping changes | Not hot path | Stable API gate | User-facing safe messages | App owns field naming policy. |
| `github.com/aatuh/api-toolkit/v4/httpcache` | Package docs | Compile checked | Direct tests | Add before header parser expansion | Not hot path | Stable API gate | Conditional request header handling | App owns cacheability and privacy. |
| `github.com/aatuh/api-toolkit/v4/httpx` | Package docs | Compile checked | Direct tests | Add before content negotiation changes | Baseline exists | Stable API gate | Problem Details and JSON response safety | App owns status code semantics. |
| `github.com/aatuh/api-toolkit/v4/httpx/identity` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Identity propagation and spoofing boundaries | App owns authenticated principal source. |
| `github.com/aatuh/api-toolkit/v4/httpx/recover` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Panic values must not leak | App owns logging and incident policy. |
| `github.com/aatuh/api-toolkit/v4/idempotent` | Package docs | Compile checked | Direct tests | Add before replay/hash parser changes | Baseline exists | Stable API gate | Replay, conflicts, key secrecy | App owns durable storage and tenant scoping. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/apikey` | Package docs | Compile checked | Direct tests | Add before header parsing expansion | Not hot path | Stable API gate | Secret handling and scope checks | App owns key issuance, rotation, and storage. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/authz` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Deny by default | App owns role model. |
| `github.com/aatuh/api-toolkit/v4/middleware/auth/tenant` | Package docs | Compile checked | Direct tests | Add before tenant source parsing changes | Not hot path | Stable API gate | Tenant mismatch must fail closed | App owns tenant source of truth. |
| `github.com/aatuh/api-toolkit/v4/middleware/deprecation` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Header values and dates | App owns deprecation lifecycle. |
| `github.com/aatuh/api-toolkit/v4/middleware/idempotency` | Package docs | Compile checked | Direct tests | Add before storage key or replay changes | Baseline exists | Stable API gate | Raw idempotency keys and replay bodies | App owns durable store and opt-outs. |
| `github.com/aatuh/api-toolkit/v4/middleware/json` | Package docs | Compile checked | Direct tests | Add before content-type parser changes | Not hot path | Stable API gate | Missing or invalid content types | Exclude non-JSON routes. |
| `github.com/aatuh/api-toolkit/v4/middleware/maxbody` | Package docs | Compile checked | Direct tests | Not required | Baseline exists | Stable API gate | Oversized bodies and multipart limits | App owns per-route sizes. |
| `github.com/aatuh/api-toolkit/v4/middleware/querylimits` | Package docs | Compile checked | Direct tests | Add before query parser changes | Baseline exists | Stable API gate | Query cardinality and memory bounds | App owns route-specific limits. |
| `github.com/aatuh/api-toolkit/v4/middleware/ratelimit` | Package docs | Compile checked | Direct tests | Not required | Baseline exists | Stable API gate | Bounded keys and dev bypasses | App owns shared store choice. |
| `github.com/aatuh/api-toolkit/v4/middleware/secure` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Headers must match deployment context | App owns CSP and TLS termination policy. |
| `github.com/aatuh/api-toolkit/v4/middleware/timeout` | Package docs | Compile checked | Direct tests | Not required | Baseline exists | Stable API gate | Streaming and optional writer interfaces | App owns deadlines and route opt-outs. |
| `github.com/aatuh/api-toolkit/v4/middleware/trace` | Package docs | Compile checked | Direct tests | Add before trace header parser changes | Not hot path | Stable API gate | Trace IDs must be bounded | App owns trace propagation policy. |
| `github.com/aatuh/api-toolkit/v4/negotiation` | Package docs | Compile checked | Direct tests | `FuzzParseAcceptAndContentType` smoke | Not hot path | Stable API gate | Header parser edge cases | App owns media-type policy. |
| `github.com/aatuh/api-toolkit/v4/operations` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Operation IDs and status visibility | App owns persistence and authorization. |
| `github.com/aatuh/api-toolkit/v4/ports` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate with ports freeze | Interface ownership and compatibility-sensitive surfaces | Prefer app-owned or package-local interfaces for new code. |
| `github.com/aatuh/api-toolkit/v4/queryparams` | Package docs | Compile checked | Direct tests | `FuzzParseCollectionQuery` smoke | Baseline exists | Stable API gate | Filter/sort bounds and validation errors | App owns field allow-lists. |
| `github.com/aatuh/api-toolkit/v4/routecontracts` | Package docs | Compile checked | Direct tests | Add before contract parser expansion | Baseline exists | Stable API gate | Unsafe write metadata and admin route policy | App owns route inventory completeness. |
| `github.com/aatuh/api-toolkit/v4/routepolicy` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Route metadata must fail closed | App owns policy mapping. |
| `github.com/aatuh/api-toolkit/v4/scheduler` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Background work, cancellation, and leases | App owns job idempotency and storage. |
| `github.com/aatuh/api-toolkit/v4/scheduler/migrations` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Compatibility-only | Migration failure and rollback safety | Use only for v3 compatibility. |
| `github.com/aatuh/api-toolkit/v4/securityprofile` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Stable API gate | Global profiles must not hide route caveats | App owns auth checks and opt-outs. |
| `github.com/aatuh/api-toolkit/v4/specs` | Package docs | Compile checked | Direct tests | Add before OpenAPI parser changes | Baseline exists | Stable API gate | Public contract accuracy | App owns source OpenAPI quality. |
| `github.com/aatuh/api-toolkit/v4/swagstub` | Package docs | Compile checked | Direct tests | Not required | Not hot path | Compatibility-only | Docs UI exposure | Use only for v3 tooling compatibility. |
| `github.com/aatuh/api-toolkit/v4/upload` | Package docs | Compile checked | Direct tests | `FuzzDecodeMultipartMetadata` smoke | Not hot path | Stable API gate | Filename, content-type, size, and memory bounds | App owns scanning and persistence. |
| `github.com/aatuh/api-toolkit/v4/webhooks` | Package docs | Compile checked | Direct tests | `FuzzHMACVerifierSignatures` smoke | Baseline exists | Stable API gate | Signature verification and replay windows | App owns duplicate suppression and provider schema validation. |

## Production Checklist

Before standardizing on any stable package:

- Confirm the row above has package docs, compile-checked examples, direct
  tests, and a benchmark/fuzz decision that fits the route risk.
- Read the package doc and any linked guide for when-not-to-use caveats.
- Check `VERSIONING.md`, `docs/api-inventory.md`, and release notes for
  compatibility-sensitive behavior.
- Review the security column for request parsing, authentication, tenant,
  replay, streaming, observability, or operator-only surfaces.
- Add service-owned tests for authorization, tenant isolation, provider
  contracts, storage durability, backups, load, and rollout behavior.

The production-readiness overview lives in
[production-readiness.md](production-readiness.md).
