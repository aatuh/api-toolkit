# API Change Request

## Package

Name the module, package, import path, and affected public symbol(s).

## Environment And Adoption Path

- api-toolkit version or commit:
- Go version:
- OS/architecture:
- Library, adapter, CLI, generated-project, or application-owned path:
- Current support tier:

## Change Type

- New exported identifier:
- Behavior change:
- Deprecation:
- Breaking change:
- Example or exact exception:
- Release note or changelog entry:

## Compatibility Evidence

Explain how this preserves the stable v3 API promise or why it requires a future
major version.

## Risk Context

- Concurrency or cancellation impact:
- Security-sensitive input, authorization, secret, provider, or resource-boundary impact:
- Dependency or generated-output impact:
- Sanitized reproduction or motivating example (if applicable):

## Tests And Docs

List required tests, examples, docs, release notes, and API compatibility checks.

## Ports Review

If this adds or changes `ports`, include the design evidence required by
`docs/stable-core.md` and `docs/ports-surface.md`.
