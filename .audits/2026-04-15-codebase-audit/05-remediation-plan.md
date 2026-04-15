# Remediation Plan

## Objective
Stabilize the toolkit's runtime behavior, converge on one explicit HTTP contract, align docs and examples with actual behavior, and close the highest-risk test gaps before further feature work.

## Inputs
This plan consolidates issues from:
- `01-bug-hunter-report.md`
- `02-rest-api-expert-report.md`
- `03-code-review-audit-report.md`
- `04-final-merged-report.md`

## Issue inventory
### Confirmed defects
- Duplicate default Prometheus registration can panic during repeated bootstrap or recorder initialization.
- Panic recovery corrupts responses after partial writes.
- JSON media-type validation accepts invalid types such as `text/application/json`.
- `docs.NewHandler(nil)` permits runtime panics.

### Contract and behavior issues
- Strict JSON mode emits plain-text `415` instead of Problem Details.
- Strict JSON mode applies content-type enforcement too broadly.
- `401` and `403` semantics are inconsistent across JWT, role, and tenant middleware.
- Spec-first example error schemas do not match actual RFC 9457 runtime behavior.
- List parsing silently coerces malformed inputs instead of rejecting them.
- Strict bootstrap profile omits query-limit middleware despite docs positioning it as baseline hardening.
- Error responses lack correlation identifiers even though logs and tracing provide them.
- Timeout behavior is documented like an enforced resource limit, but the implementation is only a context deadline.

### Structural and documentation issues
- Docs and examples overstate or misstate some default behaviors.
- Documentation around hardening baseline and module expectations is not fully aligned with code.
- Nil-dependency construction patterns likely exist outside docs handlers as well.

### Assurance and risk gaps
- No direct tests for bootstrap composition.
- No direct tests for JWT middleware.
- No direct tests for recovery behavior.
- No direct tests for docs handler construction and nil behavior.
- Weak coverage for list/query contract behavior.
- No direct tests for scheduler behavior.
- Buffered response writers may break optional `http.ResponseWriter` interfaces such as `http.Flusher`, `http.Hijacker`, `http.Pusher`, and `io.ReaderFrom`.
- Endpoint nil-dependency behavior should be audited beyond docs handlers, especially health handlers.

## Execution strategy
### Phase 1 - Stop the runtime failures
Goal: remove the confirmed defects that can panic, corrupt responses, or silently accept invalid input.

Scope:
- Fix duplicate Prometheus registration.
- Fix recovery middleware partial-write behavior.
- Fix JSON media-type parsing and strict `415` behavior.
- Reject or safely handle nil docs manager construction.

Why first:
- These are evidence-backed failures.
- They affect shared library consumers immediately.
- They are the highest-risk defects relative to patch size.

Exit criteria:
- Repros for all four confirmed defects fail before the patch and pass after the patch.
- New regression tests exist for each defect.
- `make finalize` passes.

### Phase 2 - Converge the HTTP contract
Goal: make the toolkit expose one predictable client-visible contract across middleware, helpers, bootstrap defaults, and examples.

Scope:
- Decide and implement JSON content-type policy.
- Standardize `401` versus `403` semantics across auth-related middleware.
- Decide whether malformed list/query inputs are rejected or normalized, then implement that consistently.
- Align spec-first example errors with RFC 9457 runtime behavior.
- Decide whether correlation IDs belong in error responses and apply the decision consistently.

Why second:
- Even after the confirmed bugs are fixed, downstream services would still inherit ambiguous semantics.
- Contract changes are easier once the runtime bugs are removed and covered by tests.

Exit criteria:
- A short written contract exists for JSON enforcement, auth semantics, list/query behavior, and error envelope shape.
- Code, examples, and docs all match that contract.
- Added tests cover representative happy path and failure path behavior.

### Phase 3 - Align defaults, hardening, and docs
Goal: make documented hardening claims match the real default behavior.

Scope:
- Decide timeout model: advisory context deadline or enforced wall-clock timeout.
- Implement the chosen timeout behavior or rewrite docs to describe the actual contract precisely.
- Reconcile `ProfileStrictAPI` with query-limit documentation.
- Audit README and security docs for any remaining mismatches.
- Review endpoint constructors for nil-dependency behavior beyond docs handlers.

Why third:
- These are important, but some are policy choices that should follow the Phase 2 contract decisions.

Exit criteria:
- README, security docs, and examples describe only behaviors that are verified in code.
- Default profiles and docs no longer contradict each other.
- Nil-construction behavior is consistent across endpoint packages.

### Phase 4 - Expand confidence and close unresolved risks
Goal: prevent regression and either prove or dismiss the remaining unproven risks.

Scope:
- Add direct tests for bootstrap, JWT, recovery, docs handlers, list/query helpers, and scheduler.
- Add optional-interface compatibility tests for buffering response writers.
- Decide whether idempotency and OpenAPI response capture wrappers must preserve `Flusher`, `Hijacker`, `Pusher`, and `ReaderFrom`, then implement or document the result.
- Add health-handler nil-dependency tests and fixes if required.

Why fourth:
- By this point the main behavior changes are in place, so the confidence pass can lock them down and clean up edge cases.

Exit criteria:
- Previously untested central packages have direct tests.
- Optional-interface support is either implemented or explicitly documented as unsupported.
- Remaining open risks are reduced to known, acceptable limitations rather than unknown behavior.

## Delivery rules
- Work ticket-by-ticket, not as one large patch set.
- After each ticket, run `make finalize`.
- If `make finalize` proves too expensive for a ticket-sized loop, run the smallest equivalent targeted checks during development, then run `make finalize` before closing the ticket.
- Create a Conventional Commit after each completed ticket.
- Do not mark tickets complete before checks pass.
- Keep docs and examples synchronized with implementation changes in the same ticket where practical.

## Recommended sequencing
1. Fix default metrics initialization panic.
2. Fix recovery partial-write corruption.
3. Fix JSON media-type parsing and `415` output.
4. Fix nil docs manager behavior.
5. Standardize auth semantics.
6. Decide and implement list/query contract.
7. Align spec-first example and error docs.
8. Decide timeout model and reconcile docs and defaults.
9. Add query-limit behavior to strict profile or narrow the docs.
10. Add correlation identifiers to problem responses if adopted.
11. Audit remaining endpoint constructors and buffering wrappers.
12. Complete the direct test expansion.

## Success criteria
- No known confirmed defect remains open.
- The repo documents one explicit HTTP error and validation contract.
- Bootstrap defaults, examples, and docs agree.
- Central composition paths have direct tests.
- The remaining limitations, if any, are deliberate and documented rather than accidental.
