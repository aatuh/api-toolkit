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
| `package-summary.tsv` | Stable-package dashboard plus enforced contrib floor rows. Columns include `api_status`, `test_status`, `floor_env`, `floor_percent`, `observed_percent`, and `branch_notes`. |
| `root.func` | Function-level root module coverage from `go tool cover -func`. |
| `contrib.func` | Function-level contrib module coverage from `go tool cover -func`. |
| `root.log` | Root package test coverage output used for package floor extraction. |
| `contrib.log` | Contrib package test coverage output used for package floor extraction. |

`API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make release-evidence` also runs
`coverage-check`, so publication evidence records the coverage gate log beside
lint, vuln, gosec, API compatibility, docs, tests, race, and fuzz evidence.

## Mutation Smoke

`GOTOOLCHAIN=local make mutation-smoke` runs a non-blocking mutation-testing
experiment against a small stable-core package set. It mutates boolean literals
and simple comparison/logical operators in a temporary copy of the repository,
then runs the affected package tests for each mutant.

The default package set is `./binding,./queryparams,./negotiation,./webhooks`.
Tune a local run with:

| Variable | Default | Purpose |
| --- | --- | --- |
| `MUTATION_PACKAGES` | `./binding,./queryparams,./negotiation,./webhooks` | Comma or whitespace separated `./` package patterns. |
| `MUTATION_LIMIT` | `12` | Maximum mutants to execute. |
| `MUTATION_TIMEOUT` | `30s` | Per-mutant test timeout. |
| `MUTATION_OUT` | `.ci-result/mutation/mutation-smoke.tsv` | TSV report path under the repository root. |

The TSV columns are `package`, `file`, `rule`, `original`, `replacement`,
`status`, and `duration_ms`. A `survived` mutant means the package tests still
passed after the source change; treat that as a weak-assertion review prompt.
`mutation-smoke` is intentionally opt-in and non-blocking, so it is not part of
`finalize`, `audit-check`, `release-check`, or `release-evidence`.

## Required Mutation Gate

`GOTOOLCHAIN=local make mutation-check` is the required CI gate for selected
critical parser and input-boundary packages: binding, query parameters,
negotiation, and webhooks. It runs three deterministic source mutants per
selected package and requires a 75% killed-mutant rate. A timeout or tool setup
error fails the gate; the TSV report is still written to
`.ci-result/mutation/mutation-check.tsv` and retained by CI for review.

The floor is a test-assertion signal, not a coverage target. The fixture suite
includes a known binding parser comparison mutation, so a survivor there is a
direct regression in parser-test strength rather than an excuse to inflate line
coverage. Broader `mutation-smoke` remains available for exploratory
weak-assertion review.

## Fuzz Failure Artifacts

`make fuzz` runs short deterministic smoke fuzzing on each pull request and the
scheduled `nightly` workflow runs the longer 60-second pass. It covers binding,
query parsing, negotiation, uploads, webhook signatures, proxy identity,
idempotency hash and replay metadata, and `FuzzRequestAndResponseValidation`
for OpenAPI request/response validation.

On a fuzz failure, CI uploads only the minimized `testdata/fuzz/` corpus paths
for those packages for seven days. It does not upload environment files, test logs, or the workspace. Fuzz inputs must use synthetic values; do not seed a
corpus with credentials, customer-like payloads, or production request bodies.

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

## Package Coverage Dashboard

`package-summary.tsv` is generated from `docs/package-classification.tsv` for
every root package classified as `stable` or `compatibility-only`. Each row
reports:

| Column | Meaning |
| --- | --- |
| `module` | `root`, `contrib`, or aggregate row source. |
| `package` | Import path, or `(aggregate)` for module totals. |
| `api_status` | Stability classification from `docs/package-classification.tsv`. |
| `test_status` | Test classification from `docs/package-classification.tsv`. |
| `floor_env` | Environment variable controlling the package floor, or `not-enforced`. |
| `floor_percent` | Active floor percentage, or `not-enforced`. |
| `observed_percent` | Statement coverage from `go test -cover`, `no-statements`, `no-test-files`, or `not-reported`. |
| `branch_notes` | Branch-relevant review prompt for the package risk area. |

Rows with `floor_env=not-enforced` are still visible in release evidence, but
they are not release-blocking coverage floors. Branch notes are prompts for
reviewing negative paths and risk-specific behavior; they do not replace direct
tests for malformed input, auth failures, size limits, lifecycle failures, or
other branch behavior.

## Public Trend

[Package coverage trend](coverage-trend.md) compares every dashboard package
across published releases. Its source file, `docs/coverage-trend.tsv`, records
the exact tag commit, module, package status, and observed statement coverage
for each snapshot.

After the release commit's `make coverage-check` passes, update the trend in the
release pull request before tagging:

```sh
COVERAGE_TREND_RELEASE=vX.Y.Z \
COVERAGE_TREND_COMMIT="$(git rev-parse --verify HEAD)" \
GOTOOLCHAIN=local make coverage-trend-record
GOTOOLCHAIN=local make coverage-trend-check
```

The trend documents coverage movement. It is not a substitute for the existing
floor gate and it does not make a percentage increase a release criterion.
