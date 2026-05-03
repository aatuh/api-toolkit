# Package Documentation Standard

Audience: maintainers updating public Go package docs for stable core packages,
compatibility surfaces, and high-use contrib adapters or integrations.

## Minimum package-doc content

Each public `doc.go` should answer these questions in plain Go doc prose:

| Field | Minimum expectation |
| --- | --- |
| Purpose | What task or boundary the package owns. |
| Primary abstractions | The main types, constructors, middleware, helpers, or adapters a caller starts with. |
| Stability status | Whether the package is stable core, compatibility-only, supported-adapter contrib, experimental contrib, wrapper-only, test-only, generated, tooling, or excluded. |
| Common construction path | The normal constructor or setup path when one exists. |
| Safety caveats | Fail-closed behavior, dangerous bypasses, compatibility-sensitive surfaces, or production-only constraints. |
| Examples | Pointer to `docs/cookbook.md`, `contrib/examples/README.md`, or a package-specific example when useful. |

Keep the package comment concise. Do not turn package docs into a second README.
Use links to canonical docs when the topic is already covered elsewhere.

## Inventory policy

Do not keep a historical inventory as the source of truth. The current package
classification lives in `docs/package-classification.tsv`, the stable package
surface lives in `VERSIONING.md`, and `GOTOOLCHAIN=local make docs-check`
enforces placeholder-doc regressions.

When adding or promoting a public package:

- add or update `doc.go` in the package.
- update `docs/package-classification.tsv` and `VERSIONING.md` when stability
  changes.
- link to `docs/cookbook.md` or `contrib/examples/README.md` when the package
  has a task recipe or runnable example.
- avoid changelog-style package-doc inventory here; release notes own dated
  history.

The 2026-05-03 documentation remediation added full package docs for recent
stable packages that were missing richer package comments: `apiclient`,
`apitest`, `idempotent`, `oauth2`, `routepolicy`, and `upload`.

## Quality gate

`GOTOOLCHAIN=local make docs-check` includes a docscheck rule that fails when a
public `doc.go` reintroduces the placeholder `Package X provides X utilities`
shape.
