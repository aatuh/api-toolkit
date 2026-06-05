# Dependency Risk Disposition

This file records reviewer disposition for vulnerability findings that are
imported by dependencies but not called by this repository.

## Current govulncheck disposition

Current imported-but-not-called count: `0`.

Owner decision: release evidence does not fail solely because findings are imported but not called.
Reviewers must confirm there are `0` called vulnerabilities and inspect the
imported-only IDs before publishing.

The active disposition manifest is header-only while the imported-only count is
`0`. Add rows back to `docs/vulnerability-dispositions.tsv` only when current
`govulncheck` evidence reports imported-but-not-called IDs.

Per-ID disposition: `docs/vulnerability-dispositions.tsv` is the
machine-readable owner and expiry manifest. Release evidence dynamically checks
the current imported-only IDs against this manifest and records:

- `vulnerability_evidence.missing_disposition_count`
- `vulnerability_evidence.expired_disposition_count`
- `vulnerability_evidence.disposition_issues`

Upgrade plan: keep `govulncheck` in `make release-evidence`, review
`release-check-summary.json` `vulnerability_evidence`, require zero missing and
expired imported-only dispositions, and upgrade or replace the owning contrib
dependency when an upstream fix is available, when a disposition expires, or
when the finding becomes called by repository code.

## V39 advisory ownership map

The v39 imported-only advisories were owned by the contrib module dependency
graph and were burned down by upgrading the affected modules to fixed versions.

| Advisory | Owning dependency | Found in | Fixed in | Owning paths | V39 outcome |
| --- | --- | --- | --- | --- | --- |
| `GO-2026-4772` | `github.com/jackc/pgx/v5` | `v5.8.0` | `v5.9.0` | Direct contrib dependency used by `contrib/adapters/pgxpool`, `contrib/adapters/txpostgres`, `contrib/adapters/migrate`, `contrib/migrator`, and `contrib/scheduler/postgres`. | Upgraded contrib to `github.com/jackc/pgx/v5 v5.9.0`; no active disposition row remains. |
| `GO-2026-4771` | `github.com/jackc/pgx/v5` | `v5.8.0` | `v5.9.0` | Same pgx contrib database ownership path as `GO-2026-4772`. | Upgraded contrib to `github.com/jackc/pgx/v5 v5.9.0`; no active disposition row remains. |
| `GO-2026-4762` | `google.golang.org/grpc` | `v1.78.0` | `v1.79.3` | Indirect contrib dependency pulled through OpenTelemetry OTLP gRPC tracing and grpc-gateway-generated example support. | Forced contrib graph to `google.golang.org/grpc v1.79.3`; no active disposition row remains. |

Evidence locations:

- `release-check-summary.json` `vulnerability_evidence`
- `.ci-result/release-evidence/logs/vuln.log`
- `docs/vulnerability-dispositions.tsv`

The contrib module owns most third-party integration exposure. Imported-only
findings in contrib dependencies remain release-review risks, not stable API
compatibility signals. If future evidence reports imported-only IDs again,
restore non-expired manifest rows with owner, review date, expiry date, and an
upgrade trigger before accepting release evidence.

## Dependency PR SLA

Dependabot is configured weekly for root Go modules, contrib Go modules, and
GitHub Actions. The SLA for dependency PRs is:

| PR age or type | Required action |
| --- | --- |
| Security update, critical or high | Review immediately; merge, replace, or document a blocker before the next release evidence run. |
| Security update, medium or low | Triage within 7 days; merge when tests pass or document why the advisory is not called. |
| Routine update open 14 days | Add an owner and a short disposition: merge window, blocked reason, replacement plan, or close rationale. |
| Routine update open 30 days | Refresh or close; do not let stale dependency PRs accumulate without an owner decision. |

Required triage notes:

- affected module and package,
- whether the dependency is root, contrib, tooling, or generated-service-only,
- test command run or failing check,
- release impact,
- owner and next review date.

Use `docs/vulnerability-dispositions.tsv` only for current imported-but-not-called
vulnerability IDs. Normal version-bump PRs should carry their disposition in the
PR and, when release-relevant, in `docs/release-notes.md`.

## Current dependency PR triage snapshot

Reviewed on: `2026-06-05`.

Open Dependabot PRs at review time:

| PR | Created | Labels | Disposition |
| ---: | --- | --- | --- |
| `#16` | `2026-05-22` | `dependencies`, `github_actions` | Routine GitHub Actions update. Under the 14-day SLA at review time; review after this branch lands because release evidence is being refreshed in the same window. |
| `#17` | `2026-05-22` | `dependencies`, `go` | Routine contrib Go dependency update. Under the 14-day SLA at review time; run contrib tests and drift/release-note checks before merge. |
| `#18` | `2026-05-22` | `dependencies`, `github_actions` | Routine GitHub Actions update. Under the 14-day SLA at review time; verify actions audit and release workflow behavior before merge. |
| `#19` | `2026-05-22` | `dependencies`, `go` | Superseded by the local contrib `golang.org/x/net` floor update required by govulncheck in this backlog pass; close or let Dependabot rebase after this branch lands. |
| `#20` | `2026-05-22` | `dependencies`, `go` | Routine contrib test dependency update. Under the 14-day SLA at review time; run contrib tests before merge. |
| `#21` | `2026-05-22` | `dependencies`, `go` | Routine contrib Markdown dependency update. Under the 14-day SLA at review time; run contrib email/markdown tests and `make vuln` before merge. |
| `#22` | `2026-05-22` | `dependencies`, `go` | Superseded by the local contrib `golang.org/x/text` update pulled by the `x/net` security floor; close or let Dependabot rebase after this branch lands. |
| `#23` | `2026-05-29` | `dependencies`, `github_actions` | Routine CodeQL action update. Under the 14-day SLA at review time; verify actions audit and code scanning workflow behavior before merge. |

No open dependency PR was older than 14 days at review time. All open
dependency PRs had ecosystem labels. Re-check this snapshot before the next
release because it records external GitHub state, not a repository invariant.
