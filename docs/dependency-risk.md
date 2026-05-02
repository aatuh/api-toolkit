# Dependency Risk Disposition

This file records reviewer disposition for vulnerability findings that are
imported by dependencies but not called by this repository.

## Current govulncheck disposition

Current imported-but-not-called count: `0`.

Owner decision: release evidence does not fail solely because findings are
imported but not called. Reviewers must confirm there are `0` called
vulnerabilities and inspect the imported-only IDs before publishing.
Release evidence does not fail solely because findings are imported but not called.

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
