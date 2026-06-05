# Test Coverage Evidence

Audience: maintainers and release reviewers checking package-level coverage
signals before merging or publishing.

Coverage is a review signal, not a substitute for behavior, contract, race, or
fuzz tests. The repository uses coverage to make weak areas visible and to keep
high-risk package floors from drifting down silently.

## Commands

Use the same command locally and in CI:

```sh
GOTOOLCHAIN=local make coverage-check
```

The command writes coverage evidence under `.ci-result/coverage/`:

| Artifact | Purpose |
| --- | --- |
| `summary.md` | Aggregate root and contrib totals; CI appends this to the GitHub step summary. |
| `package-summary.tsv` | Package-level floor environment variable, configured floor, and observed coverage for every enforced package floor. |
| `root.func` | Function-level root module coverage from `go tool cover -func`. |
| `contrib.func` | Function-level contrib module coverage from `go tool cover -func`. |
| `root.log` | Root package test coverage output used for package floor extraction. |
| `contrib.log` | Contrib package test coverage output used for package floor extraction. |

`API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-evidence` also runs
`coverage-check`, so publication evidence records the coverage gate log beside
lint, vuln, gosec, API compatibility, docs, tests, race, and fuzz evidence.

## Floors

The aggregate defaults are:

| Module | Environment variable | Default floor |
| --- | --- | ---: |
| root | `ROOT_COVERAGE_MIN` | `70.0` |
| contrib | `CONTRIB_COVERAGE_MIN` | `52.0` |

Package-specific floors live in `scripts/coverage_check.sh`. The generated
`package-summary.tsv` is the reviewable package-level summary for the current
run; `docs/coverage-hardening-backlog.md` explains when a floor may be raised.

Do not raise a floor just to improve the number. Raise it only after behavior
tests covering the relevant risk are merged and the focused package test plus
`make coverage-check` pass with the new threshold.
