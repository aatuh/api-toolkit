# Adopter Review

Use this public issue template when you tried `api-toolkit` in a service,
scaffold, upgrade, or design review and want to report adoption friction.

Do not include secrets, tokens, private URLs, customer data, proprietary schemas,
or vulnerability details. Use `SECURITY.md` for private security reports.

## Adoption Path

- Existing service:
- Generated scaffold:
- Contrib adapter:
- Version or commit:
- Go version:
- OS/architecture:
- Current support tier:

## API Friction

Describe the package, option, interface, import path, generated file, or release
step that was hard to understand or hard to use.

## Missing Docs

Name the docs, examples, package comments, migration notes, or troubleshooting
entries that would have helped.

## Migration Pain

Describe friction from `net/http`, chi, another toolkit, v2 to v3, a generated
service update, or a dependency upgrade.

## Runtime And Risk Context

- Concurrency, retries, cancellation, or performance context:
- Security-sensitive or compatibility-sensitive impact (do not include private details):
- Sanitized logs or errors that would help reproduce the friction (optional):

## What Worked

List the parts that were clear, useful, or safe enough to keep using.

## Requested Outcome

State whether you want a docs change, example, API review, migration note,
compatibility test, or no code change yet.
