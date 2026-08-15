# ADR 0002: V5 Module Decomposition

Status: Accepted for v5 planning; not implemented or released.

Audience: maintainers and release reviewers deciding how v5 separates the core,
optional modules, CLI, and generated applications.

## Context

The current repository contains a broad root module and a broad contrib module.
That makes an otherwise small HTTP API dependency graph inherit unrelated
provider, database, observability, and generator dependencies. The focused v5
core decision requires independent module and release boundaries before any v5
release is published.

## Decision

Retain a monorepo initially, but publish independent Go modules with independent
tags, changelogs, owners, and compatibility baselines. A monorepo preserves
atomic cross-module changes and shared test fixtures; it does not imply a shared
version or compatibility promise.

| Module role | Planned import path | Tag prefix | Owner | Support tier and cadence |
| --- | --- | --- | --- | --- |
| Core library | `github.com/aatuh/api-toolkit/v5` | `v5.` | Core API team | Stable; patch security fixes and scheduled minor releases. |
| CLI and generator | `github.com/aatuh/api-toolkit/cmd/api-toolkit/v5` | `cmd/api-toolkit/v5.` | CLI team | Supported tooling; releases independently of core. |
| PostgreSQL adapters | `github.com/aatuh/api-toolkit/adapters/postgres/v5` | `adapters/postgres/v5.` | Adapter team | Supported adapter; releases for driver/security fixes and contract changes. |
| Redis adapters | `github.com/aatuh/api-toolkit/adapters/redis/v5` | `adapters/redis/v5.` | Adapter team | Supported adapter; releases for client/security fixes and contract changes. |
| Provider adapters | `github.com/aatuh/api-toolkit/adapters/providers/v5` | `adapters/providers/v5.` | Provider adapter team | Supported or experimental per package classification; provider-driven cadence. |
| Observability adapters | `github.com/aatuh/api-toolkit/adapters/observability/v5` | `adapters/observability/v5.` | Observability team | Supported or experimental per package classification; exporter-driven cadence. |
| Optional runtime features | `github.com/aatuh/api-toolkit/runtime/v5` | `runtime/v5.` | Runtime team | Supported only where a feature contract has direct evidence. |
| Examples and reference applications | application-local module paths under `examples/` | none | Example and application owners | Non-library; app-owned lifecycle and no public compatibility promise. |

The runtime module is the home for complex opt-in behavior such as durable
idempotency, scheduler, operations, and buffered hard timeouts. A feature only
belongs there when it cannot meet the focused-core dependency and policy
boundary. Generated code is an application asset, not a published library
module.

## Dependency Direction

```text
core/v5 <- runtime/v5 <- optional adapters
   ^           ^
   |           +-- CLI may generate references to released module versions
   +-------------- generated applications choose released modules explicitly
```

Core imports the standard library only at runtime. Optional modules may import
core and narrowly scoped shared contracts, but no optional module may be
imported by core. PostgreSQL, Redis, provider, and observability modules must
not import one another merely for convenience. Shared contracts move into core
only when they satisfy the public API review policy; otherwise the consuming
module owns the interface.

## Release, Compatibility, and Security Rules

- Each module has its own `go.mod`, changelog section, SBOM/license report,
  vulnerability review, owner, support tier, and release evidence.
- The tag prefix in the table is mandatory. A release checker must reject a
  tag whose module path, tag prefix, version, commit, or module metadata does
  not agree.
- Stable modules run API-diff checks from their own latest verified compatible
  tag. Supported adapters run contract and drift checks without inheriting the
  core stable promise. Experimental modules do not claim compatibility.
- A module publishes compatible patch/minor releases only from its own clean,
  verified baseline. Major changes require a module-specific migration record.
- Security support follows [SECURITY.md](../../SECURITY.md): intake, triage,
  disclosure, and backport decisions are recorded per affected module. A
  provider or adapter issue never silently upgrades the core support promise.
- Deprecations name the owning module, replacement import path, earliest
  removal major, and migration document. The owning module retains the
  compatibility shim until that documented boundary or a security exception.

## Migration Process

`ARC-004` and `ARC-005` create the new module paths; `ARC-006` reduces the
root; and `ARC-007` supplies deterministic import rewriting and a dry run. A
v4 consumer moves only after the destination module is published, its version
is pinned, and its migration documentation compiles without repository-local
replacements. Generated projects record the selected CLI and module versions.

## Rejected Alternatives

| Alternative | Rejection reason |
| --- | --- |
| Keep one root and one broad contrib module | Core consumers would continue to inherit unrelated dependency and release risk. |
| Move each module to a separate repository immediately | It would introduce repository, access-control, CI, and migration churn before module contracts are proven. |
| Version every module together | Independent modules would still be coupled to unrelated release timing and vulnerability exposure. |
| Put all optional runtime behavior in core | It contradicts the focused v5 core and pulls operational policy into basic HTTP adoption. |
| Treat generated applications as supported libraries | Generated code has application-specific ownership and cannot carry a toolkit-wide compatibility promise. |

## Risks and Preconditions

- Cross-module change coordination can become harder; release tooling must
  expose the exact compatible module matrix.
- A premature split can leave users without a migration destination; no v4
  surface is removed until the destination is implemented and documented.
- Module tags, APIs, and support tiers can drift; `ARC-008` must make each
  stable module's baseline and API gate fail closed.
- Separate repositories remain an option only after independent ownership,
  release cadence, and contribution flow have evidence. This ADR deliberately
  does not authorize a repository split.
