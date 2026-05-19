# Governance

Audience: maintainers configuring repository protections, release approvals,
and ownership rules for api-toolkit.

## Required Protections

- Protect `master` and release branches.
- Require pull requests before merge.
- Require CODEOWNERS review using `.github/CODEOWNERS`.
- Require the `ci`, `codeql`, and `scorecard` workflow results that apply to
  the change.
- Require `make docs-check`, `make coverage-check`, `make test-race`, and the
  API compatibility check from CI before merge.
- Protect `v*` release tags so only release maintainers can create or update
  them.
- Do not publish a release from local dirty-tree audit evidence.

Maintainers can run the optional authenticated verifier with
`make github-governance-check`. The command uses `gh api` when available to
check branch protection, required status checks, CODEOWNERS review, force-push
and deletion protection, and tag rulesets. It skips cleanly when `gh` is not
installed or authenticated, and it is not part of `finalize` or required PR CI.

## Release Approval

Release reviewers use `docs/release-runbook.md` as the command source of truth
and `docs/release-review.md` as the reviewer checklist. A release is acceptable
only when clean publication evidence records `publication_eligible=true`, all
release checks pass, artifact expectations are satisfied, and the draft release
assets verify.

The first v3 release line may compare against `v2.1.0` only when recording the
intentional v2 to v3 major-version breakage evidence. v3 patch and minor
releases compare against the latest published v3 tag, currently `v3.0.1`.
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
