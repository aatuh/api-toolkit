# Deprecation Policy

Audience: maintainers planning stable API replacements and release reviewers
checking migration evidence.

Deprecation is a compatibility promise, not a cleanup shortcut. A deprecated v3
symbol remains available until the earliest documented major version unless a
security exception requires otherwise.

## Required Format

Every deprecation entry must include:

| Field | Meaning |
| --- | --- |
| Symbol | Fully qualified package symbol, such as `example.OldSymbol`. |
| Since | Release or date when the deprecation became public. |
| Replacement | Preferred symbol, package, or app-owned pattern. |
| Removal earliest major | Earliest major version where removal may happen. |
| Migration snippet | Short before/after snippet or link to a focused guide. |
| Release note | Link or pointer to the release note that announced it. |

Source comments should use Go's `Deprecated:` convention so pkg.go.dev and
`go doc` expose the status.

## Active Deprecation Register

| Symbol | Since | Replacement | Removal earliest major | Migration snippet | Release note |
| --- | --- | --- | --- | --- | --- |
| None currently documented in source comments. | n/a | n/a | n/a | n/a | n/a |

Compatibility-sensitive but not source-deprecated surfaces are tracked in
[ports-surface.md](ports-surface.md),
[v3-compatibility-roadmap.md](v3-compatibility-roadmap.md), and
[api-inventory.md](api-inventory.md). Use those documents before deciding
whether to add a `Deprecated:` source comment.

## Rules

- Do not deprecate without a replacement or explicit app-owned alternative.
- Do not remove deprecated v3 API before a major version unless a documented
  security exception requires it.
- New deprecations must update `docs/api-inventory.md`,
  `docs/release-notes.md`, and this register in the same change.
- Examples should teach the replacement, not the deprecated path.
- Compatibility-only packages may remain active without source deprecation when
  the goal is to preserve v3 users while steering new users elsewhere.
