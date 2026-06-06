# Governance

Audience: maintainers configuring repository protections, release approvals,
and ownership rules for api-toolkit.

The repository identity is small Go HTTP/API building blocks first. Scaffold and
contrib adapter work is optional and must not expand the stable root promise
without the API review and compatibility evidence required by `VERSIONING.md`
and `docs/stable-core.md`.

## Required Protections

- Protect `master` and release branches.
- Require pull requests before merge.
- Require CODEOWNERS review using `.github/CODEOWNERS`.
- Require at least one approving review for non-maintainer pull requests.
- Require the CI jobs that apply to the change:
  - `ci / test`, including `make coverage-check`, `make test-race`, and
    `make vuln`.
  - `ci / lint`, including `make lint`.
  - `ci / governance`, including `make docs-check`,
    `make v3-readiness-check`, and pull-request contrib drift/release-note
    checks.
  - `ci / api-check`, including `make release-api-check` against the pull
    request base or push predecessor.
  - `ci / fuzz`, including `make fuzz`.
  - `dependency-review / dependency-review`, which fails pull requests that
    introduce high or critical vulnerable dependencies or dependencies outside
    the configured license policy.
  - `codeql` and `scorecard` workflow results when those workflows are enabled
    for the repository.
- Keep the scheduled `nightly` workflow enabled for longer fuzzing, generated
  scaffold integration, dependency vulnerability checks, and benchmark smoke.
  It is production-readiness evidence, not a required pull-request gate.
- Protect `v*` release tags and contrib module `contrib/v*` release tags so
  only release maintainers can create or update them.
- Do not publish a release from local dirty-tree audit evidence.

Settings that are not visible in the repository must be treated as external
state. Maintainers should verify them with the GitHub UI or
`make github-governance-check` before publication review, and attach the output
when repository settings are accessible.

Maintainers can run the optional authenticated verifier with
`make github-governance-check`. The command uses `gh api` when available to
check branch protection, required status checks, CODEOWNERS review, force-push
and deletion protection, and tag rulesets for both `refs/tags/v*` and
`refs/tags/contrib/v*`. It skips cleanly when `gh` is not installed or
authenticated, and it is not part of `finalize` or required PR CI.

## PR Review Discipline

Repository branch protection should require pull requests, CODEOWNERS review,
and at least one approving review for non-maintainer pull requests. Maintainer
direct pushes are allowed only for tightly scoped maintenance, release
preparation, or urgent security work, and they still need a self-review before
release evidence is accepted.

Maintainer self-review checklist for direct pushes:

- confirm the change is tied to a backlog item, security fix, release step, or
  narrowly scoped maintenance task,
- run the narrowest validation command that proves the change and record it in
  the commit or release notes when relevant,
- check compatibility, security, dependency, generated-output, and docs impact,
- ensure `.audits`, `.trash`, local evidence, secrets, and generated scratch
  files are not staged,
- create a focused Conventional Commit,
- before publishing a release, verify the direct-push commits are covered by
  release evidence and reviewer checklist entries.

Treat branch review settings as external GitHub state. When repository settings
are accessible, attach `make github-governance-check` output or GitHub ruleset
evidence during release review.

## Code Scanning Merge Protection

Repository rulesets must require code scanning results for pull requests into
`master`. Configure the branch ruleset with:

| Setting | Required value |
| --- | --- |
| Rule | Require code scanning results |
| Required tool | CodeQL |
| Alerts threshold | Errors and Warnings or stricter |
| Security alerts threshold | High or higher or stricter |

This rule is separate from required status checks. The CodeQL workflow in
`.github/workflows/codeql.yml` enables analysis on `push`, `pull_request`, and
schedule, uploads results with `security-events: write`, and must stay enabled
so the ruleset has CodeQL results to evaluate.

Treat code scanning ruleset state as external GitHub settings. Before release
publication, maintainers must verify the active branch ruleset in GitHub UI or
via the REST API includes a `code_scanning` rule for CodeQL with at least the
thresholds above. GitHub documents that ruleset merge protection does not apply
to merge queue groups or Dependabot pull requests analyzed by default setup, so
those exceptions require ordinary maintainer review and CI evidence.

## OpenSSF Scorecard Target

The `scorecard` workflow publishes OpenSSF Scorecard results for the public README badge,
API report, SARIF artifact, and code-scanning upload. The
repository target is Scorecard `>= 8`. A lower public score is a
supply-chain-governance finding: remediate it before release publication or
record an explicit release-review disposition that explains why the lower score
is accepted for that release.

## Public Repository Metadata

GitHub About metadata should reinforce the library-first positioning and avoid
generic framework overreach.

Required description:

`Small Go HTTP API building blocks for JSON APIs, middleware, OpenAPI contracts, and service scaffolds.`

Required topics:

- `go`
- `http`
- `api`
- `middleware`
- `openapi`

Avoid broad topics such as `framework`, `microservices`, `clean-architecture`,
`ports-and-adapters`, or `developer-tools` unless the project is deliberately
repositioned and the README, alternatives, and stable-core charter change in
the same review.

## Release Approval

Release reviewers use `docs/release-runbook.md` as the command source of truth
and `docs/release-review.md` as the reviewer checklist. A release is acceptable
only when clean publication evidence records `publication_eligible=true`, all
release checks pass, artifact expectations are satisfied, and the draft release
assets verify.

The first v3 release line may compare against `v2.1.0` only when recording the
intentional v2 to v3 major-version breakage evidence. v3 patch and minor
releases compare against the latest published v3 tag named by
`docs/release-runbook.md`.
When GitHub repository settings are accessible, reviewers should attach
`make github-governance-check` output as optional publication-review evidence.

## Supported Adapter Governance

Contrib remains outside the stable core API promise. Packages classified as
`supported-adapter` require direct tests, package docs, drift coverage in
`docs/contrib-api-drift-packages.txt`, a behavior row in
`docs/supported-adapter-contracts.tsv`, and release-note review for behavior or
runtime asset changes. Experimental packages stay experimental until all of
that evidence exists.

Live provider checks are opt-in only. They must not be added to default
`finalize`, required pull-request CI, or release evidence unless a future
governance change explicitly promotes them.
