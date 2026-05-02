# Dependency Risk Disposition

This file records reviewer disposition for vulnerability findings that are
imported by dependencies but not called by this repository.

## Current govulncheck disposition

Current imported-but-not-called count: `3`.

Owner decision: release evidence does not fail solely because findings are
imported but not called. Reviewers must confirm there are `0` called
vulnerabilities and inspect the imported-only IDs before publishing.
Release evidence does not fail solely because findings are imported but not called.

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

Evidence locations:

- `release-check-summary.json` `vulnerability_evidence`
- `.ci-result/release-evidence/logs/vuln.log`
- `docs/vulnerability-dispositions.tsv`

The contrib module owns most third-party integration exposure. Imported-only
findings in contrib dependencies remain release-review risks, not stable API
compatibility signals.
