# Backlog

Project: api-toolkit audit remediation

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Fix confirmed runtime defects [x]

Description: Remove the evidence-backed failures that can panic, corrupt responses, or silently accept invalid input in normal toolkit use.

### Ticket E1-T1 - Make default metrics initialization safe [x]

Description: Eliminate duplicate Prometheus collector registration panics in default bootstrap and recorder construction paths.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/middleware/metrics/metrics.go`, `contrib/bootstrap/profile.go`, and `contrib/bootstrap/http.go`
- add a regression test that proves repeated initialization no longer panics

### Ticket E1-T2 - Prevent recovery from corrupting committed responses [x]

Description: Update panic recovery so it does not append a second payload after headers or body bytes have already been written.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `httpx/recover/recover.go`
- add tests for panic-before-write and panic-after-partial-write

### Ticket E1-T3 - Fix JSON media-type validation [x]

Description: Replace substring-based JSON media-type acceptance with exact parsing and valid `+json` handling.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/json/json.go`
- add tests that reject `text/application/json` and accept `application/json; charset=utf-8` and `application/problem+json`

### Ticket E1-T4 - Fail closed for nil docs handlers [x]

Description: Make docs handler construction or request handling deterministic when no docs manager is provided.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `endpoints/docs/handlers.go`
- prefer rejecting invalid construction over runtime panic

## Epic E2 - Converge the HTTP contract [x]

Description: Define and implement one stable client-visible contract across middleware, helpers, bootstrap defaults, and examples.

### Ticket E2-T1 - Standardize strict JSON error behavior [x]

Description: Ensure strict JSON mode returns RFC 9457 Problem Details for unsupported media types and only enforces JSON where that policy is intended.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- align `middleware/json` with the RFC 9457 guarantees described in `README.md`
- document the method and body policy explicitly

### Ticket E2-T2 - Standardize auth status semantics [x]

Description: Define and implement a consistent `401` versus `403` policy across JWT, role, and tenant middleware.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/auth/jwt`, `middleware/auth/authz`, and `middleware/auth/tenant`
- include missing-auth, malformed-auth, authenticated-but-forbidden, and missing-tenant cases

### Ticket E2-T3 - Choose and implement the list-query contract [x]

Description: Decide whether malformed pagination, filters, and sorts are rejected or normalized, then implement that behavior consistently in helpers and examples.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `endpoints/list/list.go` and `contrib/examples/pagination/main.go`
- if malformed inputs are rejected, use field-level Problem Details consistently

### Ticket E2-T4 - Align spec-first examples with runtime errors [x]

Description: Update the spec-first example and generated assets so documented error schemas match actual runtime RFC 9457 responses.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/examples/spec-first/openapi.json`, example handlers, and any generated assets
- keep `validation.fields` and related compatibility extensions in mind

### Ticket E2-T5 - Add response-side correlation support [x]

Description: Decide whether request identifiers should appear in problem responses or headers, then implement that policy consistently.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- align request logging, tracing, and error responses
- keep the label-cardinality policy unchanged for metrics

## Epic E3 - Align hardening defaults and documentation [x]

Description: Make default profiles, docs, and examples describe the same actual behavior.

### Ticket E3-T1 - Decide the timeout model [x]

Description: Choose whether request timeouts are advisory context deadlines or enforced wall-clock response limits, then implement and document that choice.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/timeout/timeout.go`, `securityprofile/profile.go`, `README.md`, and `docs/security.md`
- include at least one test that demonstrates the documented contract

### Ticket E3-T2 - Reconcile strict-profile query-limit behavior [x]

Description: Either add query-limit enforcement to the strict bootstrap profile or narrow the docs so they no longer claim it is part of the default baseline.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/bootstrap/profile.go`, `README.md`, and `docs/security.md`

### Ticket E3-T3 - Audit and fix nil-construction patterns across endpoint handlers [x]

Description: Review endpoint packages for nil dependency construction paths and make behavior consistent across docs, health, and similar handlers.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- start with `endpoints/health`
- prefer constructor-time validation over deferred panic

### Ticket E3-T4 - Correct documentation drift [x]

Description: Update README, security docs, and examples so they only describe verified behavior and do not overstate baseline guarantees or module expectations.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover timeout language, query-limit baseline language, and any remaining contract mismatches

## Epic E4 - Expand tests and close open risks [x]

Description: Add direct coverage for under-tested central packages and either prove or dismiss the remaining unresolved risks.

### Ticket E4-T1 - Add bootstrap composition tests [x]

Description: Add direct tests for strict and dev profile construction, middleware composition, and metrics behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/bootstrap/profile.go` and `contrib/bootstrap/http.go`

### Ticket E4-T2 - Add JWT trust-boundary tests [x]

Description: Add direct tests for malformed authorization headers, required claims, trusted-proxy skip behavior, and configuration validation in JWT middleware.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/auth/jwt/middleware.go`

### Ticket E4-T3 - Add endpoint-construction tests [x]

Description: Add direct tests for docs and health handler construction, nil dependency behavior, and middleware side effects.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `endpoints/docs` and `endpoints/health`

### Ticket E4-T4 - Add list and JSON contract tests [x]

Description: Add tests that lock down chosen pagination, filtering, sorting, and JSON content-type behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/json`, `endpoints/list`, and the pagination example

### Ticket E4-T5 - Verify buffering wrapper compatibility [x]

Description: Prove or dismiss the risk that idempotency and OpenAPI response capture wrappers break optional `http.ResponseWriter` interfaces, then implement or document the result.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `response_writer/capture.go`, `middleware/idempotency`, and `contrib/middleware/openapi`
- either preserve `Flusher`, `Hijacker`, `Pusher`, and `ReaderFrom`, or document unsupported behavior explicitly

### Ticket E4-T6 - Add scheduler coverage [x]

Description: Add direct tests for scheduler startup, repeated execution, last-run gating, and recorder interactions.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `scheduler/scheduler.go` and related helpers

## Epic E5 - Clear remaining finalize blockers [ ]

Description: Resolve the remaining repository-wide lint and static-analysis issues uncovered during `make finalize` so the full quality gate passes cleanly.

### Ticket E5-T1 - Remove remaining `noctx` test violations [ ]

Description: Replace the remaining `httptest.NewRequest` usages in root and contrib test files with `httptest.NewRequestWithContext`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `middleware/auth/tenant/tenant_test.go`, `contrib/middleware/requestlog/requestlog_test.go`, and `contrib/middleware/requestlog/requestlog_bench_test.go`

### Ticket E5-T2 - Resolve migrator staticcheck findings [ ]

Description: Replace the remaining `WriteString(fmt.Sprintf(...))` patterns in the migrator with direct formatted writes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/migrator/migrator.go`

### Ticket E5-T3 - Resolve intentional outbound-call `gosec` findings [ ]

Description: Either harden or explicitly annotate the remaining intentional outbound HTTP call sites so `gosec` no longer flags them as unresolved SSRF issues.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/adapters/httpclient/client.go`
- prefer explicit rationale or validation over silent suppression

### Ticket E5-T4 - Resolve migrate command log-injection finding [ ]

Description: Remove or explicitly justify the remaining `gosec` finding in the migrate command’s error path.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` is unavailable
- ensure the quality check covers testing, formatting, linting, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- cover `contrib/cmd/migrate/migrate.go`
