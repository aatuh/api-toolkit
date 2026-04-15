# Backlog

Project: api-toolkit

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Restore retry and execution safety [x]

Description: Eliminate the two high-severity runtime defects that can strand idempotency keys or run the same scheduled job concurrently.

### Ticket E1-T1 - Add idempotency failure-path regression tests [x]

Description: Add focused tests in `middleware/idempotency` covering `5xx`, panic, and store-write failure paths so retry semantics are locked before changing the implementation.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Assert that failed requests do not leave stale `InFlight` state behind.
- Cover both immediate retry behavior and replay behavior after successful completion.

### Ticket E1-T2 - Fix idempotency cleanup after failed requests [x]

Description: Change the idempotency middleware so downstream `5xx`, panic, and persistence failure paths clear or replace `InFlight` reservations instead of blocking retries until TTL expiry.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer an explicit terminal failure path over passive TTL cleanup.
- Preserve current replay behavior for successful terminal responses.

### Ticket E1-T3 - Add scheduler overlap regression tests [x]

Description: Add a blocking-job test in `scheduler` proving the runner does not start a second execution of the same job while the first execution is still running.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Use explicit synchronization instead of sleeps wherever possible.
- Verify sequential execution rather than approximate timing.

### Ticket E1-T4 - Prevent same-job overlap in scheduler.Runner [x]

Description: Add per-job in-flight protection or execution leasing so a scheduled job cannot overlap with itself unless overlap is later made an explicit opt-in behavior.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep current interval-based skip behavior only where it still makes sense after adding in-flight protection.
- Default behavior must be non-overlapping.

### Ticket E1-T5 - Document changed retry and scheduling semantics [x]

Description: Update package docs and release notes to describe the new idempotency failure behavior and scheduler non-overlap guarantee so downstream users understand the observable behavior changes.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Call out the prior `409`-until-TTL behavior as historical behavior.
- Include upgrade notes if any users relied on concurrent scheduler runs.

## Epic E2 - Make docs and OpenAPI behavior truthful [x]

Description: Ensure docs endpoints, OpenAPI responses, and docs configuration flags behave exactly as the public contract implies.

### Ticket E2-T1 - Define the missing-spec contract for docs endpoints [x]

Description: Choose and document the repository-wide behavior when no authoritative OpenAPI document exists, favoring an explicit error such as `404` or `501` over a synthetic success response unless placeholder mode is made an explicit opt-in.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Record the decision before changing handlers so package docs and tests follow one clear contract.
- Treat local-demo convenience as opt-in, not silent production default.

### Ticket E2-T2 - Enforce docs enable flags or remove unsupported config [x]

Description: Implement `EnableHTML`, `EnableJSON`, and `EnableYAML` handling in the docs manager and handlers, or remove those flags from the public surface if they are not intended to be supported.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not keep half-implemented configuration in exported types.
- Keep status codes and content-type behavior predictable when an endpoint is disabled.

### Ticket E2-T3 - Remove misleading placeholder spec responses [x]

Description: Stop returning a fabricated OpenAPI document with `200 OK` when no real spec is available, or gate placeholder generation behind an explicit local-only configuration path.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Do not silently hardcode `localhost` server entries in production-facing responses.
- Preserve the existing provider and file-loading paths for real specs.

### Ticket E2-T4 - Add docs contract tests and refresh docs [x]

Description: Add tests for disabled docs endpoints and missing-spec behavior, then update README and package docs so the published behavior matches the implementation exactly.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Cover both HTML and OpenAPI JSON surfaces.
- Include release notes for any change from placeholder `200` to explicit error responses.

## Epic E3 - Harden outbound adapters and retry headers [ ]

Description: Improve adapter unhappy-path handling and HTTP retry semantics so integrations fail clearly and clients receive accurate backoff guidance.

### Ticket E3-T1 - Add Resend adapter regression tests [ ]

Description: Add tests for malformed `2xx` JSON, empty success bodies, upstream non-`2xx` responses, and stalled requests in `contrib/adapters/resend`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Use `httptest.Server` and deterministic timeouts.
- Keep the tests local to the adapter package so failures stay easy to diagnose.

### Ticket E3-T2 - Reject malformed success responses from Resend [ ]

Description: Change the Resend adapter so a `2xx` response that cannot be decoded into a valid success payload returns an error instead of reporting success with an empty message ID.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Preserve current handling for valid success payloads and upstream error bodies.
- Keep the returned error actionable for callers.

### Ticket E3-T3 - Add bounded default timeout behavior to Resend [ ]

Description: Replace the implicit `http.DefaultClient` fallback with a bounded default client or explicit timeout option so stalled upstream calls cannot hang indefinitely.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep caller-supplied clients supported.
- Reuse existing configuration patterns if the repo already has a preferred HTTP client option style.

### Ticket E3-T4 - Fix Retry-After rounding for distributed rate limits [ ]

Description: Round retry delays up when converting limiter durations into the `Retry-After` header so sub-second retry windows do not become `Retry-After: 0`.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Add a regression test for `0 < retryAfter < 1s`.
- Keep the HTTP header conservative even if internal limiter precision stays sub-second.

## Epic E4 - Align documentation, examples, and extension contracts [ ]

Description: Remove the remaining low-severity mismatches so docs, examples, and extension points describe the system as it actually behaves.

### Ticket E4-T1 - Align migrator override docs with runtime behavior [ ]

Description: Either implement later-directory override semantics in `contrib/migrator` or update comments and documentation to state clearly that duplicate version-direction pairs are rejected.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer the lower-risk path unless there is a strong need for directory overrides.
- Add or update tests to match the final contract.

### Ticket E4-T2 - Unify pagination example validation behavior [ ]

Description: Update the pagination example and any supporting helpers so invalid `limit` inputs produce one coherent client-facing `400` response shape rather than competing validation formats.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Prefer one validation owner for `limit`.
- Keep the example aligned with recommended toolkit composition.

### Ticket E4-T3 - Refresh examples and package docs after behavior changes [ ]

Description: Update examples, package comments, and user-facing docs so they reflect the corrected idempotency, scheduler, docs/OpenAPI, migrator, and pagination behaviors.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Keep examples executable and minimal.
- Avoid documenting intended behavior that the code does not enforce.

## Epic E5 - Lock in regression protection and release safety [ ]

Description: Close the remediation work with repo-level verification, compatibility notes, and regression protection so the fixes stay durable.

### Ticket E5-T1 - Ensure new edge-case coverage is part of normal verification [ ]

Description: Verify that the new regression tests for idempotency, scheduler overlap, docs behavior, Resend, and rate limiting run under the existing repo quality targets and do not rely on one-off commands.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Reuse the existing `Makefile` targets rather than inventing a parallel workflow.
- Keep test additions proportionate to the risk they cover.

### Ticket E5-T2 - Publish upgrade notes for observable contract changes [ ]

Description: Add changelog or release-note entries covering idempotency retry semantics, scheduler non-overlap behavior, docs endpoint changes, and any configuration cleanup that affects existing consumers.

Implementation rules:

- implement the ticket in the smallest sensible step
- run `make finalize` after completing the ticket, or an equivalent quality toolkit if `make finalize` becomes unavailable
- ensure the quality check covers testing, formatting, linting, vulnerability scanning, API compatibility, race checks, fuzz smoke checks, and other relevant validation for the repository
- create a git commit immediately after the ticket is complete
- use Conventional Commits style for the commit message
- update the ticket checkmark from `[ ]` to `[x]` only after the ticket is actually complete
- update the epic checkmark from `[ ]` to `[x]` only when all child tickets are complete

Notes:

- Call out any shift from placeholder `200` behavior to explicit errors on docs endpoints.
- Explain changes that improve retry safety and execution safety so adopters can validate them during upgrade.
