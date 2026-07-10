# Package Security Review

Audience: maintainers and reviewers assessing a package-level change before it
is merged or released.

Use this checklist for **each affected package**, not once for an entire module
or pull request. An affected package is an import path in
`docs/package-classification.tsv` whose code, public contract, configuration,
generated output, or operational behavior changes. Use
`docs/package-owners.tsv` to identify the maintainer and test owner. A change
that touches several packages needs a separate review record for each one.

This is a review procedure, not a claim that every historical package change
has been reviewed. The pull request or release evidence must retain the
package-specific results and links to the evidence used.

## When It Is Required

Complete this review before merging a package change that:

- adds or changes an externally reachable input, configuration value, file,
  network target, provider callback, storage key, or generated-service route;
- changes authentication, authorization, tenant scope, sessions, API keys,
  webhooks, idempotency, admin surfaces, or dangerous bypasses;
- changes body/header/query limits, retries, timeouts, queues, caches, locking,
  concurrency, or other resource-use behavior;
- changes errors, Problem Details, logs, metrics, traces, audit metadata,
  OpenAPI examples, release evidence, or documentation containing sensitive
  values; or
- adds or promotes a public package, supported adapter, provider integration,
  generator, or CLI command.

For a narrow non-security change, record why every security-sensitive section
is not applicable. Do not use a module-wide "not applicable" result.

## Review Record

Copy this record once per affected import path into the pull request or release
review. Use `pass`, `not applicable`, or `follow-up required` for every row.
`not applicable` and `follow-up required` require a short rationale and an
evidence link; unresolved follow-ups block a security-sensitive release.

```text
Package: <import path from docs/package-classification.tsv>
Change: <commit, pull request, or release-note reference>
Maintainer owner: <docs/package-owners.tsv value>
Test owner: <docs/package-owners.tsv value>
Reviewer: <name or team>
Date: <YYYY-MM-DD>

| Area | Outcome | Rationale and evidence |
| --- | --- | --- |
| Threat model | <pass/not applicable/follow-up required> | <threat, mitigation, test or doc link> |
| Inputs | <pass/not applicable/follow-up required> | <boundary, validation, size/format limit, negative test> |
| Secrets | <pass/not applicable/follow-up required> | <storage/redaction/configuration evidence> |
| Authentication and authorization | <pass/not applicable/follow-up required> | <identity, tenant/owner scope, deny-path evidence> |
| Resource exhaustion (DoS) | <pass/not applicable/follow-up required> | <body/query/header, timeout, retry, queue, or concurrency evidence> |
| Data leakage | <pass/not applicable/follow-up required> | <response/error/cache/audit/example evidence> |
| Logging and observability | <pass/not applicable/follow-up required> | <log/metric/trace fields, cardinality, and redaction evidence> |
```

## Checklist

### Threat Model

- Identify the protected asset, trust boundary, attacker-controlled path, and
  failure mode introduced or changed by this package.
- Map the change to a row in [security threat model](threat-model.md), or add a
  new row when the current matrix does not cover the threat.
- State which layer owns the mitigation: stable core, contrib adapter,
  generated template, application, operator, or provider.
- Add a negative-path, contract, integration, or documentation check that would
  fail if the mitigation is removed.

### Inputs

- Enumerate every newly trusted or untrusted input: HTTP header, path, query,
  body, cookie, environment variable, file, database value, provider payload,
  queue message, or outbound target.
- Validate syntax, semantics, ownership, and defaults at the earliest boundary;
  reject malformed, missing, ambiguous, or unexpected values fail closed.
- Set and test relevant body, multipart, query, header, pagination, response
  capture, retry, timeout, queue, and concurrency bounds. Use
  [input-size threat review](input-size-threat-review.md) for HTTP and capture
  limits.
- Treat URLs, hostnames, redirect targets, and provider metadata as untrusted
  unless they come from reviewed operator configuration.

### Secrets

- Confirm credentials, tokens, API keys, session IDs, webhook signing secrets,
  DSNs, private keys, and provider secrets are never hard-coded in source,
  fixtures, examples, generated output, or release evidence.
- Keep durable secret material hashed or encrypted when the package owns
  storage; return a raw secret only when the contract explicitly requires a
  one-time creation response.
- Confirm configuration errors and diagnostics identify a setting without
  echoing its value. Document required operator configuration without placing
  real values in documentation.

### Authentication and Authorization

- Authenticate before authorization, tenant checks, idempotency, and side
  effects. Confirm unauthenticated and unauthorized paths fail closed.
- Identify the authenticated actor, tenant, organization, owner, role, scope,
  and route method that authorize the operation. Test cross-tenant and
  wrong-owner denial paths where the package handles scoped data.
- Keep product-specific policy application-owned, but verify toolkit middleware
  cannot silently bypass configured policy, trusted-proxy, or dangerous-bypass
  controls.
- Check error status and Problem Details output do not disclose whether another
  tenant, resource, credential, or role exists unless the product policy allows
  it.

### Resource Exhaustion (DoS)

- Bound CPU, memory, disk, sockets, goroutines, retries, queue depth, cache
  growth, lock waits, and response buffering for the package's workload.
- Use contexts, deadlines, cancellation, bounded cleanup, and backpressure for
  outbound work and long-running tasks. Do not retry unsafe operations without
  an explicit replay-safety contract.
- Test oversized, repeated, concurrent, slow, canceled, and unavailable
  dependency paths at the boundary the package owns.
- Document streaming, upgrade, large-download, or background-worker opt-outs
  when generic timeout or buffering middleware is unsafe.

### Data Leakage

- Inspect success responses, Problem Details, wrapped errors, caches, replay
  records, audit metadata, OpenAPI schemas/examples, generated clients, and
  exports for raw secrets, tokens, tenant data, provider payloads, internal
  topology, or unbounded user input.
- Apply an allow-list for retained metadata. Prefer stable identifiers, bounded
  status classes, and redacted summaries over raw values.
- Verify failure paths and partial writes do not return previously stored data
  to the wrong actor or tenant.

### Logging and Observability

- List emitted log, metric, trace, audit, and event-hook fields. Confirm their
  values are necessary, redacted, bounded, and low cardinality.
- Never emit raw authorization headers, cookies, bearer tokens, API keys,
  idempotency keys, session IDs, request/response bodies, webhook payloads,
  secrets, DSNs, or provider error bodies.
- Confirm logger, metric, and trace failures do not alter authorization or
  business outcomes unless the package explicitly owns a fail-closed audit
  requirement.
- Test redaction and error paths when a new observable field is introduced.

## Evidence And Release Decision

Use the package classification and owner manifests as the package identity and
ownership source of truth. Pair the review record with the smallest relevant
evidence:

| Change | Minimum evidence |
| --- | --- |
| Parser, middleware, or helper | Unit and negative-path tests for malformed, missing, unauthorized, and oversized inputs. |
| Scoped route or storage operation | Tenant/owner authorization tests, idempotency behavior where applicable, and Problem Details assertions. |
| Provider, webhook, or outbound adapter | Signature/authentication, replay, timeout, cancellation, SSRF, and redaction tests. |
| Log, metric, trace, audit, error, or OpenAPI change | Redaction and cardinality assertions plus documentation review. |
| Generated-service template | Generated-service tests and an explicit application-owner boundary review. |

Run `GOTOOLCHAIN=local make docs-check` for every checklist/documentation
change. Run the package's focused tests and the applicable security, race,
integration, or generated-service checks before accepting a `pass` result.
Security-sensitive release changes also require the [release review
checklist](release-review.md) and clean release evidence.

Use [security posture](security.md) for operational controls, [security threat
model](threat-model.md) for the repository-wide risk map, and
[negative-path test matrix](negative-path-test-matrix.tsv) when selecting
stable-package failure cases.
