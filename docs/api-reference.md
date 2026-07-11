# API Reference Index

Audience: API consumers who want a quick link from each stable root package to
pkg.go.dev and a compile-checked local example.

`VERSIONING.md`, `scripts/apicheck.sh`, and `docs/package-classification.tsv`
remain the source of truth for the stable API surface. This page is a rendered
index for the stable and compatibility-only root packages.

Maturity tier badges:

- `[stable]`: protected by the v3 stable API gate.
- `[compatibility-only]`: preserved for v3 compatibility, but not recommended
  as a new generic abstraction.

| Tier | Package | Purpose | Tested example |
| --- | --- | --- | --- |
| [stable] | [`apiclient`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/apiclient) | Client-side HTTP API helper utilities. | [example](../apiclient/example_test.go) |
| [stable] | [`apitest`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/apitest) | HTTP API response assertion helpers. | [example](../apitest/example_test.go) |
| [stable] | [`authorization`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/authorization) | Authorization context and scope helpers. | [example](../authorization/example_test.go) |
| [stable] | [`binding`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/binding) | Request binding and validation Problem Details helpers. | [example](../binding/example_test.go) |
| [compatibility-only] | [`compat/billing`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/compat/billing) | Hosted-checkout and billing compatibility types. | [example](../compat/billing/example_test.go) |
| [stable] | [`contracttest`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/contracttest) | Route, OpenAPI, and Problem Details contract-test helpers. | [example](../contracttest/example_test.go) |
| [stable] | [`email`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/email) | Provider-neutral email port types. | [example](../email/example_test.go) |
| [stable] | [`endpoints/docs`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/endpoints/docs) | OpenAPI/docs endpoint helpers. | [example](../endpoints/docs/example_test.go) |
| [stable] | [`endpoints/health`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/endpoints/health) | Liveness, readiness, and detailed health endpoint helpers. | [example](../endpoints/health/example_test.go) |
| [stable] | [`endpoints/list`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/endpoints/list) | List endpoint query helpers. | [example](../endpoints/list/example_test.go) |
| [stable] | [`endpoints/pprof`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/endpoints/pprof) | Admin-safe pprof endpoint mounting helpers. | [example](../endpoints/pprof/example_test.go) |
| [stable] | [`endpoints/version`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/endpoints/version) | Version endpoint helper. | [example](../endpoints/version/example_test.go) |
| [stable] | [`fielderrors`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/fielderrors) | Validation field error helpers. | [example](../fielderrors/example_test.go) |
| [stable] | [`httpcache`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/httpcache) | Conditional request and validator helpers. | [example](../httpcache/example_test.go) |
| [stable] | [`httpx`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/httpx) | HTTP responses, JSON helpers, and Problem Details helpers. | [example](../httpx/example_test.go) |
| [stable] | [`httpx/identity`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/httpx/identity) | Request identity resolution helpers. | [example](../httpx/identity/example_test.go) |
| [stable] | [`httpx/recover`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/httpx/recover) | Panic recovery middleware helpers. | [example](../httpx/recover/example_test.go) |
| [stable] | [`idempotent`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/idempotent) | Idempotent HTTP contract and response helpers. | [example](../idempotent/example_test.go) |
| [stable] | [`middleware/auth/apikey`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/auth/apikey) | API-key authentication and scope middleware. | [example](../middleware/auth/apikey/example_test.go) |
| [stable] | [`middleware/auth/authz`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/auth/authz) | Role authorization middleware. | [example](../middleware/auth/authz/example_test.go) |
| [stable] | [`middleware/auth/tenant`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/auth/tenant) | Tenant context middleware. | [example](../middleware/auth/tenant/example_test.go) |
| [stable] | [`middleware/deprecation`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/deprecation) | Deprecation, Sunset, and Link header middleware. | [example](../middleware/deprecation/example_test.go) |
| [stable] | [`middleware/idempotency`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/idempotency) | Idempotency middleware with token-aware release semantics. | [example](../middleware/idempotency/example_test.go) |
| [stable] | [`middleware/json`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/json) | JSON content-type and decoder middleware. | [example](../middleware/json/example_test.go) |
| [stable] | [`middleware/maxbody`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/maxbody) | Request body limit middleware. | [example](../middleware/maxbody/example_test.go) |
| [stable] | [`middleware/querylimits`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/querylimits) | Query limit parsing and enforcement. | [example](../middleware/querylimits/example_test.go) |
| [stable] | [`middleware/ratelimit`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/ratelimit) | Rate-limit middleware and headers. | [example](../middleware/ratelimit/example_test.go) |
| [stable] | [`middleware/secure`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/secure) | Security header middleware. | [example](../middleware/secure/example_test.go) |
| [stable] | [`middleware/timeout`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/timeout) | Cooperative and hard timeout middleware. | [example](../middleware/timeout/example_test.go) |
| [stable] | [`middleware/trace`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/middleware/trace) | Request trace middleware. | [example](../middleware/trace/example_test.go) |
| [stable] | [`negotiation`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/negotiation) | Content negotiation helpers. | [example](../negotiation/example_test.go) |
| [stable] | [`operations`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/operations) | Asynchronous operation response and polling helpers. | [example](../operations/example_test.go) |
| [stable] | [`ports`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/ports) | Generic logger, clock, and ID contracts. | [example](../ports/example_test.go) |
| [stable] | [`queryparams`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/queryparams) | Collection query parameter parsing helpers. | [example](../queryparams/example_test.go) |
| [stable] | [`routecontracts`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/routecontracts) | Route and OpenAPI contract registration helpers. | [example](../routecontracts/example_test.go) |
| [stable] | [`routepolicy`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/routepolicy) | Operation-derived runtime policy middleware helpers. | [example](../routepolicy/example_test.go) |
| [stable] | [`scheduler`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/scheduler) | Scheduler abstractions. | [example](../scheduler/example_test.go) |
| [compatibility-only] | [`scheduler/migrations`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/scheduler/migrations) | v3 migration compatibility helpers. | [example](../scheduler/migrations/example_test.go) |
| [stable] | [`securityprofile`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/securityprofile) | Security profile defaults and route overrides. | [example](../securityprofile/example_test.go) |
| [stable] | [`specs`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/specs) | OpenAPI spec registry and schema helpers. | [example](../specs/example_test.go) |
| [compatibility-only] | [`swagstub`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/swagstub) | v3 tooling compatibility docs registration helper. | [example](../swagstub/example_test.go) |
| [stable] | [`upload`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/upload) | Multipart upload decoding and validation helpers. | [example](../upload/example_test.go) |
| [stable] | [`webhooks`](https://pkg.go.dev/github.com/aatuh/api-toolkit/v4/webhooks) | Webhook verification and receiver helpers. | [example](../webhooks/example_test.go) |
