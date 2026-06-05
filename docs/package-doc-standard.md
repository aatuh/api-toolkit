# Package Documentation Standard

Audience: maintainers updating public Go package docs for stable core packages,
compatibility surfaces, and high-use contrib adapters or integrations.

## Minimum package-doc content

Each public `doc.go` should answer these questions in plain Go doc prose:

| Field | Minimum expectation |
| --- | --- |
| Purpose | What task or boundary the package owns. Stable core docs must include a `Purpose:` line. |
| Import or install | The exact import path for library packages, or install/setup path for tooling packages. Stable core docs must include an `Import:` or `Install:` line. |
| Primary abstractions | The main types, constructors, middleware, helpers, or adapters a caller starts with. |
| Minimal example | Pointer to a package example, `docs/api-reference.md`, `docs/cookbook.md`, `contrib/examples/README.md`, or a short inline example. Stable core docs must include an `Example:` line. |
| Errors | The main error-returning, Problem Details-writing, fail-closed, or no-hidden-error behavior a caller must know. Stable core docs must include an `Errors:` line. |
| Concurrency | Whether constructed values are immutable/reusable, request-scoped, or require caller synchronization. Stable core docs must include a `Concurrency:` line. |
| Stability status | Whether the package is stable core, compatibility-only, supported-adapter contrib, experimental contrib, wrapper-only, test-only, generated, tooling, or excluded. Stable core docs must include a `Stability:` line. |
| When not to use | A concise boundary that tells callers when `net/http`, app-owned code, a narrower helper, or a different adapter is a better fit. Stable core docs must include a `When not to use:` line. |
| Safety caveats | Fail-closed behavior, dangerous bypasses, compatibility-sensitive surfaces, or production-only constraints. |

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

`GOTOOLCHAIN=local make docs-check` includes docscheck rules that fail when a
public `doc.go` reintroduces the placeholder `Package X provides X utilities`
shape, when stable or compatibility-only core package comments omit the required
purpose, import/install, example, errors, concurrency, stability, and
when-not-to-use fields, when stable or supported-adapter package comments fall
below the minimum depth expected by this standard, or when supported-adapter
contract evidence points at an implementation file instead of the canonical
`doc.go`.
