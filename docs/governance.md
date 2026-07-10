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
- Enable GitHub Secret Scanning and push protection for supported secret
  patterns. Treat them as required merge-prevention controls, not as a
  replacement for review or safe configuration design.
- Keep the scheduled `nightly` workflow enabled for longer fuzzing, generated
  scaffold integration, generated full-profile soak checks, generated failure integration checks,
  dependency vulnerability checks, and benchmark smoke. The soak step runs
  `make generated-soak-check` with a 300-second in-process race/goroutine soak
  and three Docker-backed integration cycles to catch goroutine leaks,
  race-prone caches, and connection leaks. The failure step runs
  `make generated-failure-check` and records Redis-down, Postgres-down,
  expired API key, bad JWKS endpoint, and slow downstream timeout behavior under
  `.ci-result/generated-failure`. These are production-readiness evidence, not a required pull-request gate.
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

## Maintainer Succession And Unavailability

api-toolkit is currently a single-maintainer project. There is no automatic
successor, emergency support desk, or transfer of repository, release, or
security authority when that maintainer is unavailable. Opening an issue or
pull request does not grant anyone permission to merge, publish, change GitHub
settings, access private security reports, or represent the project.

### Temporary Unavailability

An absence pauses best-effort work; it does not relax the normal review,
security, or release rules. Contributors may open issues and pull requests, but
must not assume a merge, release, response target, or private contact route is
available. Do not create a public security issue or advisory to force an
escalation. Follow `SECURITY.md`, keep the report private, and use the documented
deployment mitigations while waiting for an authorized maintainer.

### Succession Request

After 90 consecutive calendar days with no public maintainer activity or
published availability notice, a contributor may open a public succession
request as a GitHub issue. The request must name the candidate, summarize their
relevant contributions or stewardship experience, define the requested scope
(triage, review, releases, security coordination, or ownership), and link this
policy. It must not include personal contact details, account-recovery details,
credentials, private security report details, or pressure reporters to disclose
vulnerabilities publicly.

The request is a visible record of interest, not an authorization. Only a
person who already has the necessary GitHub and project authority can add an
administrator, transfer ownership, or grant private-security access. If no
authorized person can act, this policy does not promise a repository transfer;
the candidate may maintain a clearly named fork with independent governance,
release, and security policies.

### Verified Handover

When an authorized maintainer is available to hand over responsibility, record
the new maintainer's accepted scope in the succession issue, then:

1. Grant only the repository permissions needed for that scope and retain the
   branch, tag, CODEOWNERS, and required-check protections in this document.
2. Have the incoming maintainer review `SECURITY.md`, the release runbook, open
   security reports they are authorized to access, and the current release
   evidence before making a release or security commitment.
3. Verify the repository settings with `make github-governance-check` where
   authenticated access is available, and record any external-setting gap in
   the handover issue.
4. Rotate or reissue any provider, registry, or automation credential through
   its owning service after access is confirmed. Never transfer tokens, recovery
   codes, private keys, or secrets through Git, issues, pull requests, or docs.

The release workflow uses GitHub OIDC for SBOM signing; that does not remove
the need to verify GitHub repository authority before a new maintainer publishes
a release. Until the handover is recorded, the current supported release and
published security policy remain the only official project commitments.

## Workflow Token Permissions

Every repository workflow must declare workflow-level `permissions`; omitted
scopes become `none`. Read-only validation workflows use only `contents: read`.
`permissions: read-all` and `permissions: write-all` are prohibited because
they grant unrelated repository scopes. See [GitHub's workflow permission
reference](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#permissions).

The permitted write or OIDC exceptions are purpose-bound:

| Workflow | Exception | Reason |
| --- | --- | --- |
| `codeql.yml` | `security-events: write` | Upload CodeQL SARIF results. |
| `scorecard.yml` analysis job | `security-events: write`, `id-token: write` | Upload SARIF and publish verified Scorecard results. Its workflow-level permissions are `{}`; job-level read scopes exist only for the Scorecard checks. |
| `release.yml` | `contents: write`, `attestations: write`, `id-token: write` | Create the draft release, create provenance attestations, and sign SBOMs with GitHub OIDC. |

The Actions audit rejects missing workflow-level permissions and broad
`read-all`/`write-all` defaults. Add a new scope only with a documented action
requirement, a focused workflow change, and a matching audit-contract update.

## Action Pin Refresh Policy

`.github/dependabot.yml` checks GitHub Actions from `/` weekly and opens `ci`
prefixed pull requests. Dependabot updates workflow action references and their
same-line version comments; generated Go workflow templates are source code,
so maintainers must update those template pins in the same pull request when
they use the affected action. See [GitHub's Dependabot action-update
guidance](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/auto-update-actions).

Every third-party `uses:` reference must keep a full 40-character commit SHA
and a same-line human-readable version comment, for example
`actions/checkout@<sha> # actions/checkout vX.Y.Z`. Do not replace the SHA with
a mutable tag. Before accepting a new SHA, verify that it belongs to the named
upstream action repository rather than a fork, review the action release notes
and source changes, and confirm that the comment names the reviewed version.
GitHub documents full commit SHA pinning as the immutable action-reference
option. See [GitHub's secure use reference](https://docs.github.com/en/actions/reference/security/secure-use).

For every Actions Dependabot pull request:

1. Review the Dependabot summary, upstream release notes, and the exact
   owner/repository, SHA, and same-line version comment.
2. Reassess token permissions and secret exposure. Do not add a scope, secret,
   `pull_request_target` trigger, or untrusted-event interpolation merely to
   make the action update pass.
3. Synchronize generated `actions/checkout` and `actions/setup-go` templates
   when the update changes either action; inspect generated workflow output.
4. Run `GOTOOLCHAIN=local make actions-audit`,
   `GOTOOLCHAIN=local make actions-audit-contract`, the affected generated
   scaffold tests when templates changed, and `GOTOOLCHAIN=local make docs-check`.
5. Merge only after required GitHub checks pass. Security updates follow the
   dependency remediation targets in `SECURITY.md`; defer a version update only
   with an owner, reason, review date, and removal trigger in the appropriate
   release review or disposition record.

The Actions audit rejects unpinned references, missing version comments,
deprecated provenance action comments, stale generated checkout/setup-go pins,
and broad or missing workflow permissions. It does not prove an action is safe;
the upstream review and required CI remain mandatory.

## Secret Scanning And Example Configuration

Secret scanning and push protection are external GitHub settings. Before a
release, verify in repository Security settings or organization policy that both
are active for this repository; the checked-in governance script does not claim
to verify them. A passing workflow, a clean `git diff`, or a `.gitignore` rule
does not prove that no secret was committed.

Review every changed configuration, generated artifact, deployment manifest,
fixture, and documentation example for credentials. Reject tracked `.env`,
`.env.*`, private-key, credential, or production secret files. The only
environment-file exception is a placeholder-only `.env.example`; sanitized
examples such as `secret.example.yaml` may name required keys but must never
contain live values.

Generated `saas-api`, `saas-api-full`, `dev-api`, and `saas-web` profiles emit
`.env.example` and ignore `.env` plus `.env.*`. Real values belong in the
developer's local environment or deployment secret manager. Review generated
output after scaffold changes to ensure this boundary remains true.

When a scan, reviewer, or maintainer finds an exposure, rotate or revoke the
credential before trying to edit history. Then remove it from current source,
examples, logs, releases, and deployment systems as applicable. Do not put the
secret in a public issue while reporting the incident; follow `SECURITY.md` for
private disclosure and history-remediation coordination.

## Adopter Feedback Loop

Public adopter feedback uses
`.github/ISSUE_TEMPLATE/adopter_review.md`. The template asks for the adoption
path, API friction, missing docs, migration pain, what worked, and the requested
outcome. Maintainers should convert recurring feedback into one of:

- documentation fixes,
- examples or migration notes,
- API review issues,
- compatibility tests,
- backlog items with explicit non-goals.

GitHub Discussions are disabled. The [Questions And Discussions policy in
CONTRIBUTING.md](../CONTRIBUTING.md#questions-and-discussions) makes GitHub
issues the public route for focused questions and open-ended design proposals;
keep actionable API, docs, compatibility, and migration follow-up there so it
can be linked from release review and backlog work. Public feedback must not
include secrets, tokens, private URLs, customer data, proprietary schemas, or
vulnerability details; use `SECURITY.md` for private security reports.

## Stable API Review Board

Stable root API growth requires public design review before implementation.
This applies to:

- adding a new stable root package,
- promoting an experimental, test-only, tooling, generated, or contrib package
  to stable root API,
- moving a compatibility-only package into the recommended stable-core path,
- adding a new exported interface or broad abstraction that changes the shape
  of root stable API.

Required process:

1. Open a public GitHub design issue before implementation. The issue must
   state the user problem, proposed package or symbol shape, alternatives
   considered, dependency impact, compatibility impact, security impact, and
   why app-owned or contrib-owned code is not enough.
2. Leave the issue open for a public comment window of at least 7 calendar days
   unless the change is an urgent security fix. Security exceptions must be
   documented in `docs/release-notes.md` and the release review.
3. Record maintainer approval in the issue after the comment window. Today that
   is the repository maintainer; if more maintainers are added, approval must
   include the CODEOWNER for the affected package or docs area.
4. Merge implementation only after the issue links to the PR or commit and the
   change updates `VERSIONING.md`, `docs/package-classification.tsv`,
   `docs/package-owners.tsv`, `docs/api-inventory.md`, docs/examples,
   release notes, and relevant docscheck coverage.

The review board can reject or defer stable API even when code is correct.
Accepted alternatives include contrib, app-owned interfaces, experimental
packages, compatibility-only packages, or no project change.

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

## Experimental Feature Lifecycle

Every experimental package, adapter, generator feature, or scaffold capability
must have a package-classification row, an owner row, a stated boundary, and a
review trigger. Experimental means maintained only to the extent recorded by
its owner; it is not covered by the stable root compatibility promise and must
not be presented as the default adoption path.

Graduation requires evidence appropriate to the destination:

1. To become a `supported-adapter`, the feature needs direct behavior tests,
   package documentation, a supported-adapter contract, realistic-test evidence,
   release-drift coverage, an owner, and release-note review.
2. To become stable root API, it must first satisfy the supported-adapter or
   equivalent behavior evidence where applicable, then complete the stable API
   review board process: public design issue, at least seven calendar days for
   comment, maintainer approval, compatibility evidence, examples, and release
   notes.
3. Generated-service behavior remains app-owned unless its reusable toolkit
   surface completes the same classification and review evidence; generated
   output alone is not promotion evidence.

Freeze a feature when adoption, ownership, maintenance capacity, or evidence is
insufficient to justify further work. Keep its classification explicit, change
the package-owner or release-note record to say `frozen`, state whether only
security/correctness fixes are accepted, and give the next review date or
removal trigger. A frozen feature is neither silently promoted nor treated as a
new recommended path.

Remove an experimental feature only after documenting the affected import path,
configuration, generated output, or behavior; the owner; replacement or
app-owned alternative; migration impact; and release-note entry. For a public
experimental surface, provide at least one release cycle of notice when a safe
replacement exists. Stable and compatibility-only APIs follow `VERSIONING.md`
and `docs/deprecations.md` instead; experimental status is not permission to
break a stable contract. Update package classification, ownership, examples,
docs, tests, and release evidence together so removal is visible to adopters.
