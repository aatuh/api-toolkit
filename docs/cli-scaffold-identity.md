# CLI And Scaffold Identity

Audience: maintainers and adopters deciding whether api-toolkit is primarily a
library, generator, or service framework.

api-toolkit remains library-first. The root module is the stable core library
for Go HTTP API guardrails. CLI commands, service scaffolds, generated clients,
deployment starters, and generated application templates are tooling surfaces,
not the identity of the root module.

## Decision

For the retained v4 compatibility surface:

- Keep the CLI at `github.com/aatuh/api-toolkit/contrib/v4/cmd/api-toolkit`.
- Keep generated service scaffolds under contrib tooling and generated outputs.
- Keep CLI, scaffold, and generated-service packages outside the root stable
  API promise.
- Treat generated services as app-owned code after generation.
- Do not move scaffold-only dependencies, templates, provider wiring, router
  adapters, or generated app internals into the root module.

For the implemented v5 module boundary:

- The CLI is `github.com/aatuh/api-toolkit/cmd/api-toolkit/v5`, with the
  `cmd/api-toolkit/v5.` tag prefix and its own changelog.
- It depends explicitly on the selected core version but does not import the
  contrib provider graph at runtime. Generated apps continue to record their
  explicit v4 core/contrib dependencies until corresponding v5 modules exist.
- The split does not make the root module scaffold-first. The root module
  remains the smallest useful library dependency for an existing `net/http`,
  chi, or app-owned router service.

## Current Surface Ownership

| Surface | Current location | Status | Ownership rule |
| --- | --- | --- | --- |
| Stable HTTP guardrails | `github.com/aatuh/api-toolkit/v4` | Stable core API | Release API gate and SemVer protect exported stable packages. |
| CLI and contract tools | `github.com/aatuh/api-toolkit/cmd/api-toolkit/v5` | Independently versioned tooling | Pin versions in CI and review behavior changes through the CLI changelog. |
| Service scaffolds | CLI templates and generated outputs | Tooling/generated | Generated services are app-owned code; downstream services own product changes. |
| Reference service | `examples/reference-saas-api` | Adoption evidence | Maintainers use it as checked-in scaffold evidence, not a root API contract. |
| Contrib adapters | `github.com/aatuh/api-toolkit/contrib/v4/...` | Supported-adapter, experimental, wrapper-only, tooling, or generated | Contrib remains outside the stable core API promise. |

## Split Triggers

Consider a separate CLI module or repository only if one of these becomes true:

- root adopters routinely confuse the project with a scaffold-first framework,
- release evidence for generator behavior becomes too noisy for library
  releases,
- contrib dependency footprint makes CLI installation or tooling review
  materially harder,
- downstream services need a pinned generator stream that should evolve on a
  different cadence from root packages.

Do not split only for cosmetic naming. A split creates migration work, duplicate
release evidence, and another compatibility story.

## Release And Ownership Boundary

`docs/package-owners.tsv` assigns
`github.com/aatuh/api-toolkit/cmd/api-toolkit/v5` to `cli-maintainers` with the `tooling` tier and a
`touch-scoped-tooling` release blocker. It is not a root stable package and
must never be added to the stable package list in `VERSIONING.md`.

CLI, template, generated-client, and generated-service behavior release with
the CLI module. Its independent changelog and release workflow record the
generator baseline; generated upgrade compatibility is reviewed through `make generated-upgrade-compat-check`.
Downstream services still own their generated
diffs and product behavior.

The local module split is complete, but published tag, artifact, and external
release-evidence proof remain separate from source work. See
[CLI v5 migration](cli-v5-migration.md) for the compatibility path and
EXT-005 for publication evidence.

## Guardrails For New Work

- New root packages must solve library-first HTTP/API guardrail problems.
- New generator behavior belongs in the independent CLI module, not root.
- New scaffold defaults must keep product domain, deployment topology, secrets,
  and provider account choices app-owned.
- New docs should direct existing services to `docs/library-first.md` or
  `docs/minimal-core.md` before scaffold-first material.
- New release notes should label CLI, template, generated-client, and generated
  service changes as generated scaffold or tooling impact.

Related documents:

- `docs/scaffold-first.md`
- `docs/scaffold-support.md`
- `docs/contrib-adapters.md`
- `docs/extension-module-assessment.md`
- `docs/v4-plan.md`
- `docs/cli-v5-migration.md`
- `docs/adr/0001-module-boundaries.md`
