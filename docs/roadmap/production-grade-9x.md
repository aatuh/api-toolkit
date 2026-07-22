# Production-grade 9/10 backlog

## Baseline

This backlog is based on the inspected repository state:

* Inspected local `master` at
  `a2708db2d7ddcc61c0f0106ceff44f2206ae4c99`; `origin/master` was at
  `ed73982f3d6c8da70a7f32e2995636c4feb3f6d0`.

* Published `v4.0.0` resolves to
  `3cfc8d44423029ec50516d6b857d938b75067737`. Published root and contrib
  `v4.0.1` tags resolve to `09e0117828c960453e3fb4cd028a02bc3e56ff33`.
  The `v4.0.1` modules resolve through `go list -m`, but that is not by itself
  release-authenticity evidence.

* `v4.0.0` is not an ancestor of `master`; `v4.0.1` is. The two tags have
  divergent sibling histories, so neither can be treated as the verified v4
  baseline until the release-identity incident is dispositioned.

* `make docs-check` currently reports a checksum mismatch between the contrib
  module's recorded root `v4.0.0` checksum and the fetched module, plus a
  reference-service module-tidiness failure. Treat the checksum mismatch as an
  unresolved release-integrity finding, not proof of tampering.

* Current release documentation still names `v4.0.0` as the latest release
  line despite the published `v4.0.1` tags. Historical `v3.1.2` references
  require a current-versus-historical review.

* The stable API promise covered 43 root package paths.

* CI and support policy covered Go 1.25.x and required only Linux amd64.

* Checked-in release evidence referred to a different commit from the v4 tag.

* Public response writers discarded write errors, binding could expose arbitrary
  error details, idempotency construction required a stronger interface than
  its option type declared, health constructors were inconsistent, in-memory
  rate-limit cleanup scanned state under the request-path lock, and hard
  timeout middleware buffered responses in a child goroutine.

* Several supported adapters relied primarily on fake database or hermetic
  provider evidence rather than direct real-service contracts.

* The repository was explicitly maintained by one maintainer.

Before starting implementation, confirm that these findings are still present. If the
repository has changed, update the baseline without deleting unresolved findings.

## Program baseline review (2026-07-22)

The original evidence baseline remains
`a2708db2d7ddcc61c0f0106ceff44f2206ae4c99` for the inspected local branch and
`ed73982f3d6c8da70a7f32e2995636c4feb3f6d0` for `origin/master`. The program
was entered into the maintained documentation set on 2026-07-22. Remediation
commits made after the original inspection do not erase its unresolved findings
or independently raise a score; the scorecard retains the audit baseline until
an independent review records new evidence.

---

# Backlog execution rules

1. Copy this backlog into
   `docs/roadmap/production-grade-9x.md`.
2. Keep every epic and ticket as `[ ]` until it is fully complete.
3. Change a ticket from `[ ]` to `[x]` only after:

   * The implementation is complete.
   * Acceptance criteria pass.
   * Required tests pass.
   * Documentation and release notes are updated where relevant.
   * The ticket has one merged conventional commit on the protected default
     branch.
   * The commit SHA is recorded in the GitHub issue or project item.
4. Change an epic from `[ ]` to `[x]` only after every child ticket is `[x]`.
5. Use one ticket per pull request and one final merged commit per ticket.
6. Do not combine unrelated tickets into one commit.
7. If a ticket cannot be completed atomically, split it into smaller tickets
   before implementation.
8. Every conventional commit body should contain:

```text
Refs: <ticket-id>
```

9. Breaking commits must use `!` and include:

```text
BREAKING CHANGE: <migration impact and replacement>
```

10. Settings-only work, such as enabling branch protection, must still produce
    a repository commit containing machine-readable or documented evidence of
    the applied configuration.
11. Generated files changed by a ticket belong in that ticket's commit.
12. Unless a ticket defines a narrower command, use:

```sh
GOWORK=off GOTOOLCHAIN=local make fast-check
GOWORK=off GOTOOLCHAIN=local make audit-check
```

13. Before release-sensitive tickets are marked complete, use:

```sh
API_BASE_REF=<latest-verified-supported-tag> \
  GOWORK=off \
  GOTOOLCHAIN=local \
  make release-check
```

`REL-000` must designate the verified v4 baseline before a v4 release ticket
uses it in `API_BASE_REF`.

---

# Delivery milestones

| Milestone           | Intended result                                                                    |
|---------------------|------------------------------------------------------------------------------------|
| M0: Trust repair    | Published releases are dispositioned; the verified baseline, documentation, and evidence agree. |
| M1: v4 hardening    | Non-breaking safer APIs, stronger CI, and real integration evidence.               |
| M2: v5 architecture | Focused core, independently versioned modules, and reduced compatibility burden.   |
| M3: v5 release      | Release candidate validated by external adopters and independent reviewers.        |
| M4: 90-day proof    | Maintenance, compatibility, security, and adoption claims validated after release. |

---

# [ ] EPIC PRG: Program control and handover

**Goal:** Make the backlog executable by a team without relying on undocumented
maintainer knowledge.

**Epic completion criteria:**

* Every finding is represented by a ticket.
* Every ticket has an owner role, dependency, acceptance criteria, verification,
  and required commit.
* GitHub project status matches the checked-in backlog.
* No work is marked complete without a merged commit.

## [ ] PRG-001: Add the 9/10 roadmap and evidence scorecard

**Priority:** P0
**Owner:** Technical program lead
**Size:** S
**Depends on:** None

### Work

* Add `docs/roadmap/production-grade-9x.md`.
* Add `docs/roadmap/scorecard.tsv` with these columns:

```text
area
baseline_score
target_score
current_score
owner
required_evidence
evidence_location
status
last_reviewed
```

* Include rows for:

  * Dependency-worthiness.
  * Code quality.
  * Test quality.
  * Documentation.
  * Open-source trust.
  * Ecosystem fit.
  * Scope and completeness.
  * API design.
* Add the roadmap and scorecard to `docs/README.md`.
* Record the exact baseline commits and review date.
* Create GitHub labels:

  * `epic`
  * `P0`
  * `P1`
  * `P2`
  * `area/release`
  * `area/api`
  * `area/docs`
  * `area/testing`
  * `area/security`
  * `area/cli`
  * `area/governance`
  * `breaking-change`
  * `external-review`
* Create a GitHub project with columns:

  * Backlog.
  * Ready.
  * In progress.
  * Review.
  * Blocked.
  * Done.

### Done when

* Every ticket in this backlog has a corresponding project item.
* Every ticket ID is unique and searchable.
* The scorecard has no blank owner or evidence fields.
* `docs-check` verifies that every scorecard area maps to at least one open or
  completed ticket.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
docs(roadmap): add production-grade 9x program
```

---

## [ ] PRG-002: Enforce one ticket per conventional commit

**Priority:** P0
**Owner:** Maintainer
**Size:** S
**Depends on:** PRG-001

### Work

Update `CONTRIBUTING.md` and `.github/pull_request_template.md` to require:

* One backlog ticket per pull request.
* One final conventional commit per ticket.
* Ticket ID in the commit body.
* An explicit compatibility classification:

  * No public effect.
  * Additive API.
  * Behavioral change.
  * Deprecation.
  * Breaking change.
* An explicit security classification.
* Commands and results used for verification.
* Documentation and release-note impact.
* Generated-file impact.
* Benchmark impact.
* Migration impact.

Add a repository check that verifies pull-request titles use conventional commit
syntax.

### Done when

* Invalid PR titles fail CI.
* The PR template requires the ticket ID.
* Breaking PRs require a `BREAKING CHANGE:` section.
* Maintainer documentation explains how squash merge preserves one final
  conventional commit per ticket.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make github-governance-check
```

**Required commit:**

```text
docs(contributing): require one ticket per conventional commit
```

---

# [ ] EPIC REL: Release and repository integrity

**Goal:** Make the default branch, tags, release assets, release evidence, and
documentation describe the same immutable source state.

**Epic completion criteria:**

* The latest stable tag is reachable from the default branch.
* Checked-in and published evidence names the exact release commit.
* Root and related module tags are coherent.
* Release artifacts are independently verifiable.
* A repair release has been published.

## [ ] REL-000: Disposition the v4 release-identity incident

**Priority:** P0
**Owner:** Release engineer
**Size:** M
**Depends on:** PRG-001

### Work

Investigate the published v4 release line before treating any v4 tag as a
compatibility or release-evidence baseline.

* Preserve every published tag and asset. Do not move, delete, or recreate
  `v4.0.0`, `contrib/v4.0.0`, `v4.0.1`, or `contrib/v4.0.1`.
* Record the tag commit, tree, module-proxy identity, module checksum, release
  asset checksums, and default-branch reachability for every affected tag.
* Determine why `v4.0.0` and `v4.0.1` have divergent histories and why the
  contrib module's recorded `v4.0.0` checksum differs from the fetched module.
* Treat the checksum mismatch as a release-integrity incident until evidence
  rules out an incorrect tag, artifact, or module-proxy publication. Do not
  describe it as tampering without evidence.
* Publish `docs/release-incident-v4-release-identity.md` with the timeline,
  affected versions, consumer impact, evidence, owner, reviewer, and recovery
  decision.
* Give each affected published release one final public status:

  * Verified supported baseline.
  * Superseded by a verified release.
  * Withdrawn — do not use.

* Update the GitHub release notes, `CHANGELOG.md`, README, support policy, and
  release runbook to use the same status and consumer guidance.
* Record `VERIFIED_V4_BASE_REF` in the incident document. It must identify the
  sole tag allowed as the v4 compatibility and release-evidence baseline.
* Do not reuse `v4.0.1`: it is already published. Select the next v4 version
  only after the incident decision and according to the SemVer impact of the
  repair.

### Done when

* The incident document links every relevant immutable tag, module identity,
  and release asset manifest.
* The checksum discrepancy has a documented root cause or a dated,
  independently reviewed risk disposition.
* Every affected release has one final public status and clear consumer action.
* `VERIFIED_V4_BASE_REF` is recorded and all current release documentation
  names the same baseline.
* No later v4 release ticket treats an undispositioned tag as trustworthy.

### Verification

```sh
git rev-parse v4.0.0^{} v4.0.1^{}
git merge-base v4.0.0 v4.0.1
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/v4@v4.0.1
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/contrib/v4@v4.0.1
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
docs(release): disposition v4 release identity incident
```

---

## [ ] REL-001: Reconcile `master` with the verified v4 release history

**Priority:** P0
**Owner:** Release engineer
**Size:** M
**Depends on:** REL-000

### Work

* Create a protected safety branch from the current `master`.
* Follow the `REL-000` disposition before deciding whether `v4.0.0` may be
  reconciled into `master`.
* Inventory every commit present in the selected verified v4 baseline but
  absent from `master`.
* Determine whether each commit should be:

  * Merged into `master`.
  * Reapplied with a corrected commit.
  * Explicitly superseded.
* Do not move or rewrite any published v4 tag.
* Make `VERIFIED_V4_BASE_REF` an ancestor of the default branch. Do not force a
  withdrawn tag into `master` only to satisfy reachability.
* Resolve any conflicts without dropping verified v4 API or release-evidence
  changes.
* Extend `docs/release-incident-v4-release-identity.md` with the branch
  divergence root cause and recovery.
* Add a prevention action to the release runbook.

### Done when

The verified v4 baseline is an ancestor of `origin/master`.

Also required:

* Every published v4 tag still resolves to its original commit.
* A withdrawn v4 tag is clearly marked as such and is not presented as a
  supported baseline.
* `master` passes root and contrib tests.
* No v4 public package is lost.
* The incident document contains:

  * Timeline.
  * Root cause.
  * Impact.
  * Recovery.
  * Prevention.
  * Owner.

### Verification

```sh
test -n "${VERIFIED_V4_BASE_REF:?set by REL-000}"
git merge-base --is-ancestor "$VERIFIED_V4_BASE_REF" origin/master
git fsck --full
GOWORK=off GOTOOLCHAIN=local make audit-check
API_BASE_REF="$VERIFIED_V4_BASE_REF" \
  GOWORK=off \
  GOTOOLCHAIN=local \
  make release-check
```

**Required commit:**

```text
fix(release): reconcile verified v4 history
```

---

## [ ] REL-002: Bind release evidence to the exact tagged commit

**Priority:** P0
**Owner:** Release engineer
**Size:** M
**Depends on:** REL-001

### Work

Update the release evidence generator and verifier to record and validate:

* Release tag.
* Commit SHA.
* Commit tree SHA.
* Default branch.
* Whether the tag commit is reachable from the default branch.
* Dirty, staged, untracked, and deleted file counts.
* Root module path and version.
* Contrib module path and version.
* CLI module path and version when split.
* Go version.
* Tool versions.
* Workflow identity.
* Repository identity.
* Evidence schema version.
* Evidence generation time.
* SHA-256 digest of every release asset.

The verifier must fail if:

* Evidence commit differs from `HEAD`.
* Evidence tag does not point to `HEAD`.
* The tag is not reachable from the protected default branch.
* A release asset is missing from the manifest.
* The manifest contains an unrecognized asset.
* Module versions disagree with the tag.
* Evidence is dirty or generated from an untrusted workflow identity.

Update:

* `scripts/release_evidence.sh`
* `scripts/release_artifact_verify.sh`
* Contract tests.
* `docs/release-runbook.md`
* `docs/release-manifests.md`

### Done when

* A fixture with a mismatched commit fails.
* A fixture with an unreachable tag fails.
* A fixture with a missing asset fails.
* A fixture with a modified asset fails.
* A valid clean fixture passes.
* `release-check-summary.json` names the exact tested commit.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make release-artifact-verify-fixture
test -n "${VERIFIED_V4_BASE_REF:?set by REL-000}"
API_BASE_REF="$VERIFIED_V4_BASE_REF" \
  GOWORK=off \
  GOTOOLCHAIN=local \
  make release-evidence
```

**Required commit:**

```text
fix(release): bind evidence to tagged commits
```

---

## [ ] REL-003: Enforce tag, branch, and module coherence

**Priority:** P0
**Owner:** Release engineer
**Size:** M
**Depends on:** REL-001, REL-002

### Work

Add `scripts/release_tag_consistency.sh` and a contract test.

For the current two-module layout, enforce:

* Root tag: `vX.Y.Z`.
* Contrib tag: `contrib/vX.Y.Z`.
* Root and contrib tags point to the same commit unless an ADR explicitly
  permits independent releases.
* Root `go.mod` major matches the root tag.
* Contrib `go.mod` major matches the contrib tag.
* Contrib requires a published compatible root version.
* `CHANGELOG.md` contains the release.
* Release notes contain the release.
* Current support documentation names the release.
* The release workflow's API baseline is not an older major by accident.

After module decomposition, replace the same-commit rule with the approved
per-module release policy.

### Done when

* Mismatched root and contrib tags fail.
* A wrong module path fails.
* A missing changelog entry fails.
* An outdated release baseline fails.
* A valid release candidate and stable tag both pass.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make release-tag-consistency-check
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
ci(release): enforce tag and module coherence
```

---

## [ ] REL-004: Record release-specific coverage and benchmark baselines

**Priority:** P0
**Owner:** Test lead
**Size:** M
**Depends on:** REL-002

### Work

* Record coverage for the exact repaired v4 release commit.
* Add the release to:

  * `docs/coverage-trend.tsv`
  * `docs/coverage-trend.md`
* Record benchmark metadata:

  * Commit.
  * Tag.
  * Go version.
  * OS.
  * Architecture.
  * CPU identity.
  * Benchmark flags.
  * `B/op`.
  * `allocs/op`.
  * Time per operation.
* Update `docs/benchmark-baselines.tsv`.
* Make release evidence verify that the release tag appears in both coverage and
  benchmark source files.
* Do not copy v3 values into a v4 row.
* Document packages whose code moved or was split so comparisons remain honest.

### Done when

* The v4 release has package-level root and contrib coverage rows.
* The baseline commit equals the release commit.
* Benchmark output is reproducible from documented commands.
* Release evidence fails when the coverage release row is absent.

### Verification

```sh
GOTOOLCHAIN=local make coverage-check
GOTOOLCHAIN=local make coverage-trend-check
GOTOOLCHAIN=local make benchmark-baseline-check
```

**Required commit:**

```text
test(release): record v4 quality baselines
```

---

## [ ] REL-005: Publish a verified v4 trust-repair release

**Priority:** P0
**Owner:** Release engineer
**Size:** M
**Depends on:** REL-000, REL-001, REL-002, REL-003, REL-004, DOC-001, DOC-005

### Work

Prepare the next verified v4 repair release after the `REL-000` decision.
`v4.0.1` is already published and must not be reused or retagged. Record the
selected `REPAIR_RELEASE_TAG` in the incident document; use `v4.0.2` only when
the repair is patch-compatible, otherwise select the next SemVer version that
matches the user-visible change.

The release must contain:

* Default-branch reconciliation.
* Correct v4 documentation.
* Exact commit-bound evidence.
* Coverage and benchmark snapshot.
* Root and contrib tags according to the approved policy.
* SBOMs.
* Dependency-license reports.
* Asset manifest.
* Cosign signatures and certificates.
* GitHub provenance attestations.
* Verification instructions.
* A release note that clearly states whether runtime code changed.

Publish `docs/releases/<REPAIR_RELEASE_TAG>-verification.md` containing:

* Tag.
* Commit.
* Asset digests.
* Signature verification result.
* Attestation verification result.
* Required check results.
* Known limitations.
* Reviewer identity.

### Done when

* The tag is reachable from `master`.
* Release assets download and verify using only documented commands.
* The default branch README names `REPAIR_RELEASE_TAG` as the latest verified
  v4 release and gives clear status for every affected earlier v4 tag.
* No current-state document claims a superseded or withdrawn release is the
  latest supported release.
* Root and contrib module installations resolve without `replace`.

### Verification

```sh
test -n "${REPAIR_RELEASE_TAG:?set by REL-000}"
GOWORK=off GOTOOLCHAIN=local go list -m github.com/aatuh/api-toolkit/v4@"$REPAIR_RELEASE_TAG"
GOWORK=off GOTOOLCHAIN=local go list -m github.com/aatuh/api-toolkit/contrib/v4@"$REPAIR_RELEASE_TAG"
RELEASE_TAG="$REPAIR_RELEASE_TAG" \
  GITHUB_REPOSITORY=aatuh/api-toolkit \
  RELEASE_ARTIFACT_VERIFY_MODE=publication \
  make release-artifact-verify
```

**Required commit:**

```text
chore(release): prepare verified v4 repair release
```

---

# [ ] EPIC DOC: Documentation and adoption experience

**Goal:** Make the project understandable, accurate, and safely adoptable without
requiring users to study repository governance internals.

**Epic completion criteria:**

* A new developer understands the value in 30 seconds.
* A root-only example works in five minutes.
* Every stable package documents when not to use it.
* Current-state documentation is version-correct.
* Maintainer material is separated from adopter material.

## [ ] DOC-001: Add a current-version consistency gate

**Priority:** P0
**Owner:** Documentation lead
**Size:** M
**Depends on:** REL-001

### Work

Add `scripts/version_consistency_check.sh` and contract tests.

The check should inspect:

* `README.md`
* `SECURITY.md`
* `VERSIONING.md`
* `CONTRIBUTING.md`
* `ROADMAP.md`
* Current production-readiness documents.
* Current support documents.
* Current release instructions.
* Package documentation.
* Generated documentation pages.

Allow historical major versions only in explicit locations such as:

* Changelog historical entries.
* Migration guides.
* Archived release notes.
* Compatibility fixtures.
* API diff baselines.

Use an allowlist file with comments explaining every historical exception.

### Done when

* Current guidance names the verified v4 baseline and gives every superseded or
  withdrawn v4 release its `REL-000` status.
* Current guidance cannot claim v3 is the latest release.
* Current root import examples cannot use `/v3`.
* Historical migration documents continue to pass.
* The check runs in `docs-check` and the release workflow.
* A contract fixture proves stale current-version prose fails.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
ci(docs): enforce current version references
```

---

## [ ] DOC-002: Rewrite the README around focused HTTP API guardrails

**Priority:** P1
**Owner:** Documentation lead
**Size:** M
**Depends on:** DOC-001, ARC-001

### Work

The first screen of `README.md` should contain:

1. One-sentence pitch.
2. Intended user.
3. Explicit non-goals.
4. Root-only install command.
5. Minimal existing-service example.
6. Current support statement.
7. Stability statement.
8. Optional contrib and CLI distinction.

Recommended pitch:

> `api-toolkit` provides zero-dependency, composable `net/http` guardrails
> for existing Go APIs: bounded input, RFC 9457 errors, route contracts,
> idempotency, and security-conscious middleware without replacing your
> router or application architecture.

Move these out of the primary adoption flow:

* Funding policy.
* Detailed release commands.
* Manifest file lists.
* Internal governance processes.
* Scorecard mechanics.
* Full SaaS profile inventory.
* Deep health endpoint implementation details.
* Maintainer succession policy.

Add a decision table for:

* Use root core.
* Use an optional adapter module.
* Use the CLI.
* Use plain `net/http` or chi instead.
* Use Huma, oapi-codegen, Goa, or Connect instead.

### Done when

* The project purpose is clear in the first 15 lines.
* The minimal example uses only stable root packages.
* The README states the tested Go versions.
* The README states the supported platforms.
* The README distinguishes stable core, optional modules, and generated code.
* All commands compile or execute in CI.
* No maintainer-only process dominates the first half of the README.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make example-compile-check
```

**Required commit:**

```text
docs(readme): focus adoption on http api guardrails
```

---

## [ ] DOC-003: Reorganize public documentation

**Priority:** P1
**Owner:** Documentation lead
**Size:** L
**Depends on:** DOC-001, DOC-002

### Work

Rebuild `docs/README.md` into these sections:

1. Getting started.
2. Choosing packages.
3. Core API guides.
4. Middleware safety.
5. Adapters and integrations.
6. CLI and generated projects.
7. Operations and production readiness.
8. Security.
9. Versioning and migration.
10. Contributor and maintainer documentation.
11. Historical records.

For every document:

* Assign one audience.
* Assign one canonical owner.
* Remove duplicate policy text.
* Replace duplicated prose with links to the canonical source.
* Mark historical documents as historical.
* Remove orphan documents.
* Add link checking.
* Add generated search index validation.

### Done when

* Every public document is linked from `docs/README.md`.
* Every policy has one source of truth.
* No current policy is duplicated in three or more places.
* All relative links pass.
* Adopter documents do not require reading maintainer release internals.
* Search results distinguish current and historical documentation.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
docs: reorganize public documentation
```

---

## [ ] DOC-004: Standardize package documentation and API reference

**Priority:** P1
**Owner:** Core API team
**Size:** XL
**Depends on:** ARC-002

### Work

For every stable or compatibility-only package, ensure `doc.go` documents:

* Maturity status.
* Primary use case.
* When not to use the package.
* Constructor behavior.
* Zero-value behavior.
* Nil receiver behavior.
* Context and cancellation.
* Concurrency safety.
* Resource ownership.
* Error model.
* Panic behavior.
* Performance or buffering caveats.
* Security boundary.
* Minimal example.
* Link to detailed guide where necessary.

Generate a canonical API reference index from `go list` and package comments.

Add `docscheck` rules requiring:

* Every stable package has package documentation.
* Every stable package has at least one compile-checked example.
* Every stable package has a maturity classification.
* Every stable package has an explicit production caveat.
* Every exported deprecated symbol has a replacement.

### Done when

* `go doc` is sufficient for normal package adoption.
* Every stable package has a working example.
* No package maturity is inferred from its directory.
* The API reference matches the module's exported surface.
* `pkg.go.dev` links are included for released modules.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make example-compile-check
GOWORK=off GOTOOLCHAIN=local make release-api-check
```

**Required commit:**

```text
docs(api): standardize package adoption guidance
```

---

## [ ] DOC-005: Synchronize versioning, support, security, and migration policy

**Priority:** P0
**Owner:** Release engineer
**Size:** L
**Depends on:** DOC-001, CI-001, CI-002

### Work

Synchronize:

* `VERSIONING.md`
* `SECURITY.md`
* `CHANGELOG.md`
* `ROADMAP.md`
* `docs/support-policy.md`
* `docs/production-readiness.md`
* `docs/core-readiness.md`
* `docs/release-runbook.md`
* `docs/release-notes.md`
* `docs/migration/v4.md`
* Future `docs/migration/v5.md`
* `docs/alternatives.md`

Add explicit statements for:

* Minimum supported Go version.
* Current tested Go version.
* Supported OS and architecture matrix.
* Stable module compatibility.
* Optional module compatibility.
* Generated-code ownership.
* Security backport policy.
* Maintenance response expectations.
* End-of-life policy.
* Current release line.
* Historical release lines.
* When not to use the project.

Expand alternatives to include:

* Standard `net/http`.
* chi.
* Huma.
* oapi-codegen.
* Goa.
* Connect.
* Local application helpers.

### Done when

* No current document contradicts another current document.
* Support claims exactly match CI.
* Security examples use the current release.
* Migration guides compile.
* Alternatives are specific and non-promotional.
* Generated code and optional modules have explicit compatibility boundaries.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make example-compile-check
```

**Required commit:**

```text
docs(versioning): align support and migration guidance
```

---

# [ ] EPIC CI: Toolchain, platform, and compatibility verification

**Goal:** Prove compatibility on supported Go versions and platforms, reduce test
flakiness, and test downstream consumers rather than only repository fixtures.

**Epic completion criteria:**

* Minimum and current Go releases are tested.
* Core portability is verified on multiple operating systems and architectures.
* Required checks have stable names.
* Fuzz, mutation, and timing-sensitive tests are trustworthy.
* Downstream consumer compatibility is continuously tested.

## [ ] CI-001: Test minimum and current Go releases

**Priority:** P0
**Owner:** Build engineer
**Size:** M
**Depends on:** REL-001

### Work

Update root and contrib CI to test:

* Minimum supported Go: 1.25.x.
* Current supported Go: 1.26.x.

Run at minimum:

* Unit tests.
* Build.
* Examples.
* Module verification.
* API compatibility checks where tool compatibility permits.

Use:

```text
GOTOOLCHAIN=local
```

Do not silently download another toolchain.

Update:

* Root `go.mod`.
* Contrib `go.mod`.
* Workflow matrices.
* Support policy.
* Release evidence.
* Tool installation scripts.

### Done when

* Both Go lines pass root and contrib tests.
* Release evidence records both matrix results.
* The current Go line is not merely a non-blocking experiment.
* The minimum version fails clearly when unsupported syntax is introduced.
* Dependabot or tool updates do not silently raise the minimum version.

### Verification

```sh
GOTOOLCHAIN=local go test ./...
(
  cd contrib
  GOTOOLCHAIN=local go test ./...
)
```

Run once under each supported Go release.

**Required commit:**

```text
ci(go): test supported and current toolchains
```

---

## [ ] CI-002: Add cross-platform core verification

**Priority:** P1
**Owner:** Build engineer
**Size:** M
**Depends on:** CI-001

### Work

Add required root-module jobs for:

* Linux amd64.
* Linux arm64.
* macOS arm64.
* Windows amd64.

Run:

* `go build ./...`
* `go test ./...`
* Compile-checked examples.

Keep race testing on platforms where the workflow is reliable.

Contrib may retain narrower support, but every exception must be documented per
module.

Audit scripts and tests for:

* POSIX-only shell assumptions.
* Path separator assumptions.
* File permission assumptions.
* Case-sensitive filesystem assumptions.
* Hard-coded `/tmp`.
* Executable suffix assumptions.

### Done when

* Root tests pass on all declared platforms.
* Platform support policy exactly matches the workflow.
* Platform-specific failures have documented exclusions.
* Generated projects compile on every platform claimed by the CLI.

### Verification

CI matrix plus local cross-compilation checks:

```sh
GOOS=windows GOARCH=amd64 go build ./...
GOOS=linux GOARCH=arm64 go build ./...
```

**Required commit:**

```text
ci(platform): verify core portability
```

---

## [ ] CI-003: Stabilize required quality gate names and outputs

**Priority:** P1
**Owner:** Build engineer
**Size:** M
**Depends on:** CI-001

### Work

Create stable required-check identities for:

* Unit and coverage.
* Race.
* Lint.
* Vulnerability.
* CodeQL.
* Documentation.
* Dependency boundaries.
* API compatibility.
* Fuzz smoke.
* Platform compatibility.
* Integration contracts.
* Release consistency.

Add `docs/required-checks.json` containing:

* Check name.
* Workflow file.
* Job ID.
* Whether required for PRs.
* Whether required for releases.
* Owning team.

Add a verifier that compares expected checks with GitHub branch rules using
authenticated `gh api` during governance audits.

### Done when

* Renaming a required workflow job requires updating the manifest.
* Branch-protection verification fails when a required check is absent.
* Check results are summarized in release evidence.
* Every required check has an owner.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make github-governance-check
```

**Required commit:**

```text
ci: stabilize required quality gates
```

---

## [ ] CI-004: Harden timing, fuzz, and mutation testing

**Priority:** P1
**Owner:** Test lead
**Size:** L
**Depends on:** CI-001

### Work

* Replace fragile 5 ms timing assumptions with synchronization and wider
  deadlines.
* Add repeated stress runs for timing-sensitive packages.
* Run short fuzz smoke tests on every PR.
* Run extended fuzzing on a schedule for:

  * Binding.
  * Query parsing.
  * Negotiation.
  * Uploads.
  * Webhook signatures.
  * Identity and proxy headers.
  * Idempotency hashes and replay metadata.
  * OpenAPI request and response validation.
* Persist failing fuzz inputs as artifacts.
* Promote mutation testing from an optional experiment to a blocking gate for
  selected critical packages.
* Define mutation kill thresholds based on meaningful assertions, not arbitrary
  coverage inflation.
* Update `docs/negative-path-test-matrix.tsv`.

### Done when

* Timing tests pass under `-count=100`.
* A known timing mutation is detected.
* A known parser mutation is killed.
* Fuzz crashes produce reproducible corpus files.
* Critical negative paths are represented in the matrix.
* No test relies solely on wall-clock sleep when a synchronization primitive can
  express the condition.

### Verification

```sh
go test ./middleware/timeout -count=100
GOWORK=off GOTOOLCHAIN=local make fuzz
GOWORK=off GOTOOLCHAIN=local make mutation-smoke
```

**Required commit:**

```text
test: harden timing fuzz and mutation coverage
```

---

## [ ] CI-005: Add a downstream consumer compatibility corpus

**Priority:** P1
**Owner:** Compatibility lead
**Size:** L
**Depends on:** CI-001, DOC-004

### Work

Create independently buildable downstream fixtures outside the root workspace:

1. Minimal `net/http` consumer.
2. Existing chi service.
3. Root plus idempotency consumer.
4. Root plus PostgreSQL and Redis adapter consumer.
5. Generated CLI consumer.

Each fixture must:

* Use released module versions.
* Avoid local `replace` directives during release verification.
* Compile against the previous stable minor.
* Compile against the candidate release.
* Run behavior tests for the APIs it consumes.
* Exercise migration instructions.

Add upgrade smoke jobs that test:

* Previous stable to current patch.
* Previous stable to current minor.
* v4 to v5 migration once available.

### Done when

* API-diff success is supplemented by downstream behavior tests.
* Fixtures build outside `go.work`.
* The release workflow fails when a documented upgrade path fails.
* At least two fixtures are not direct copies of generated golden files.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make upgrade-smoke-check
GOWORK=off GOTOOLCHAIN=local make downstream-compat-check
```

**Required commit:**

```text
test(compat): add downstream consumer corpus
```

---

# [ ] EPIC API: Public API and runtime safety

**Goal:** Replace hidden failures, ambiguous semantics, and oversized configuration
surfaces with explicit and composable APIs.

**Epic completion criteria:**

* Response write errors are observable.
* Unknown validation errors are not exposed to clients.
* Required field presence differs from non-zero validation.
* Idempotency requirements are visible at compile time.
* Health construction is consistent and validated.
* Rate limiting reports complete decisions and scales predictably.
* Buffered hard timeout behavior is route-explicit.

## [ ] API-001: Add error-returning HTTP response writers

**Priority:** P0
**Owner:** Core API team
**Size:** L
**Depends on:** CI-004

### Work

Add additive v4 APIs:

```go
func WriteJSONChecked(
    w http.ResponseWriter,
    status int,
    value any,
) error

func WriteProblemChecked(
    w http.ResponseWriter,
    status int,
    problem Problem,
) error
```

Define a typed write error with stages:

```go
type ResponseWriteStage string

const (
    ResponseWriteStageEncode ResponseWriteStage = "encode"
    ResponseWriteStageHeader ResponseWriteStage = "header"
    ResponseWriteStageBody   ResponseWriteStage = "body"
)
```

The checked APIs must:

* Marshal before committing headers.
* Return encoding errors without writing.
* Return body write errors.
* Never attempt a second response after a header or body write has started.
* Preserve `errors.Is` and `errors.As`.
* Avoid leaking response data in error strings.

Keep existing v4 void writers as compatibility wrappers.

For v5, rename the checked APIs to the canonical `WriteJSON` and `WriteProblem`
names and remove the void variants.

### Done when

* Encoding failure writes no partial body.
* Header failure is returned.
* First-body-write failure is returned.
* Partial-body failure is returned.
* No fallback response is attempted after commitment.
* Checked and compatibility APIs have compile-checked examples.

### Verification

```sh
go test ./httpx -race
go test ./httpx -run 'Test.*Write.*Failure' -count=100
```

**Required commit:**

```text
feat(httpx): add error-returning response writers
```

---

## [ ] API-002: Migrate repository internals to checked response writers

**Priority:** P1
**Owner:** Core API team
**Size:** XL
**Depends on:** API-001

### Work

Update internal packages to use checked writers.

For every call site, choose one explicit failure policy:

* Return the error.
* Invoke an existing `OnError` callback.
* Log through an injected logger.
* Stop processing when the connection is already committed.
* Ignore only when the API contract explicitly documents that the server owns
  connection-level write failures.

Do not add package-level global loggers.

Update tests for:

* Middleware error responses.
* Health endpoints.
* Documentation endpoints.
* Authentication failures.
* Idempotency failures.
* Rate-limit failures.
* Recovery middleware.
* Generated handlers.

Deprecate direct internal use of void response writers.

### Done when

* Repository code does not accidentally write a second response.
* High-risk middleware reports write failures through its existing error hook or
  logger.
* Void writers remain only in compatibility examples and wrappers.
* Static checks reject new internal calls to void writers.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make fast-check
GOWORK=off GOTOOLCHAIN=local make test-race
```

**Required commit:**

```text
refactor(httpx): use checked response writers internally
```

---

## [ ] API-003: Prevent unsafe validation detail exposure

**Priority:** P0
**Owner:** Security engineer
**Size:** M
**Depends on:** API-001

### Work

Change validation response construction so arbitrary `err.Error()` values are
not exposed.

Add an explicit safe-public-error contract, for example:

```go
type PublicError interface {
    error
    PublicMessage() string
}
```

Rules:

* `fielderrors.Provider` remains safe when each field message is explicitly
  classified as public.
* `PublicError` may expose `PublicMessage`.
* Unknown errors produce a generic public detail such as
  `"validation failed"`.
* Original errors remain available to application logs.
* Error types must not include secrets, SQL text, provider payloads, tokens, or
  internal URLs.

Audit every call to:

* `binding.ValidationProblem`
* `binding.WriteValidationProblem`
* `httpx.WriteProblem`
* Generated validation handlers

### Done when

* A raw SQL error is not returned to the client.
* A provider error containing a token is not returned.
* Safe field errors remain useful.
* Generated code logs internal context separately from the response.
* Security tests assert that known secret patterns never appear in response
  bodies.

### Verification

```sh
go test ./binding ./fielderrors ./httpx -race
GOWORK=off GOTOOLCHAIN=local make gosec
```

**Required commit:**

```text
fix(binding): prevent unsafe validation detail exposure
```

---

## [ ] API-004: Separate required presence from non-zero validation

**Priority:** P1
**Owner:** Core API team
**Size:** L
**Depends on:** API-003

### Work

Preserve current v4 behavior while adding explicit presence-aware decoding.

Add a required-field mode:

```go
type RequiredMode string

const (
    RequiredModeNonZero RequiredMode = "nonzero"
    RequiredModePresent RequiredMode = "present"
)
```

For v4:

* Preserve the current default where compatibility requires it.
* Let callers opt into `RequiredModePresent`.
* Document that non-pointer scalar fields cannot distinguish `null` from zero
  after ordinary decoding unless raw presence is tracked.
* Track JSON member presence before decoding.
* Track query and path presence from their source maps.

For v5:

* Make presence-aware required validation the default.
* Use separate validation for non-zero constraints.
* Document pointer and nullable field behavior.

Add table-driven and fuzz tests covering:

* Missing field.
* `null`.
* `0`.
* `false`.
* Empty string.
* Empty array.
* Empty object.
* Present pointer to zero.
* Duplicate JSON members.
* Unknown fields.
* Query parameters with empty values.
* Repeated query values.

### Done when

* Required `false` can be accepted when present.
* Required `0` can be accepted when present.
* Non-zero validation remains available as a separate rule.
* Existing v4 users retain documented compatibility.
* Migration documentation explains the v5 default change.

### Verification

```sh
go test ./binding -race
go test ./binding -run Required -count=100
GOWORK=off GOTOOLCHAIN=local make fuzz
```

**Required commit:**

```text
feat(binding): add presence-aware required fields
```

---

## [ ] API-005: Make idempotency store requirements compile-time visible

**Priority:** P1
**Owner:** Runtime API team
**Size:** M
**Depends on:** CI-005

### Work

Add an additive v4 constructor that requires the real store contract:

```go
func NewWithStore(
    store ReleasableStore,
    opts Options,
) (*Middleware, error)
```

Or introduce a dedicated constructor options type with a
`ReleasableStore` field.

Requirements:

* Construction must not accept a `Store` that is guaranteed to fail because it
  lacks token-aware release.
* Keep the old v4 constructor for source compatibility.
* Deprecate the weak constructor with a clear replacement.
* Update adapters and examples.
* Add compile-time interface assertions for every supported store.
* Add contract tests that all supported stores implement release semantics.

For v5:

* Remove the weak constructor.
* Make the stronger interface the only supported type.

### Done when

* Correct stores compile.
* Incomplete stores fail at compile time when using the new API.
* Old v4 code continues to compile with a deprecation notice.
* Every supported adapter passes the shared releasable-store contract.

### Verification

```sh
go test ./middleware/idempotency -race
(
  cd contrib
  go test ./adapters/idempotency ./adapters/idempotencyredis -race
)
```

**Required commit:**

```text
feat(idempotency): require releasable stores at construction
```

---

## [ ] API-006: Decompose idempotency configuration

**Priority:** P1
**Owner:** Runtime API team
**Size:** XL
**Depends on:** API-005

### Work

Replace the oversized conceptual configuration with focused groups:

```go
type Limits struct {
    MaxBodyBytes     int64
    MaxResponseBytes int64
}

type Retention struct {
    CompletedTTL time.Duration
    InFlightTTL  time.Duration
}

type FailurePolicy struct {
    FailOpen bool
    OnError  func(error)
}

type Observability struct {
    Logger    Logger
    OnOutcome OutcomeHandler
}

type Compatibility struct {
    KnownInFlightTTLs                  map[string]time.Duration
    FailOnInFlightTTLMismatch          bool
    FailOnClockSkewPreflight           bool
    ExposeRawLegacyKey                 bool
    LegacySink                         LegacySink
    LegacyMetricSink                   LegacyMetricSink
    LegacySampleEvery                  int
}
```

The exact names may change during API review, but the boundaries must remain:

* Core request behavior.
* Storage and retention.
* Resource limits.
* Failure policy.
* Observability.
* Legacy compatibility.

For v4:

* Add the grouped configuration.
* Keep old fields deprecated.
* Define deterministic precedence when both old and new fields are present.
* Reject ambiguous conflicting configuration.

Audit the asynchronous compatibility sink:

* If it owns a goroutine, add explicit shutdown.
* If it does not own a goroutine, document that fact.
* Test queue saturation and shutdown.
* Do not leak goroutines during repeated construction.

For v5:

* Remove deprecated flat fields.
* Move legacy compatibility into an optional compatibility package where
  feasible.

### Done when

* Normal users can configure idempotency without reading legacy migration fields.
* Conflicting old and new configuration fails clearly.
* Compatibility telemetry cannot expose raw keys by default.
* Owned background resources have an explicit lifecycle.
* Zero values and defaults are documented and tested.

### Verification

```sh
go test ./middleware/idempotency -race
go test ./middleware/idempotency -run Compatibility -count=100
```

**Required commit:**

```text
refactor(idempotency): separate compatibility options
```

---

## [ ] API-007: Normalize and validate health manager construction

**Priority:** P1
**Owner:** Runtime API team
**Size:** L
**Depends on:** CI-004

### Work

Add a canonical constructor:

```go
func NewManager(config Config) (*Manager, error)
```

Add:

* `Config.Validate() error`.
* Explicit default configuration.
* Injected clock.
* Error-returning checker registration.
* Duplicate-name rejection.
* Empty-name rejection.
* Nil-checker rejection.
* Configurable duplicate policy only if a concrete production use case exists.
* Explicit behavior when no checks are configured.
* Explicit behavior for a checker that ignores cancellation.

Recommended registration API:

```go
func (m *Manager) RegisterChecker(checker Checker) error
```

Preserve old v4 constructors as deprecated wrappers.

For v5:

* Remove inconsistent constructors.
* Return the concrete manager unless interface abstraction is required by a
  documented extension point.

### Done when

* Invalid timeout and cache values fail at construction.
* Duplicate checker names fail.
* Nil checkers fail.
* Time-based cache tests use an injected clock.
* Concurrent registration and checking are race-tested.
* No-check configuration fails closed.
* Existing v4 callers have a documented migration path.

### Verification

```sh
go test ./endpoints/health -race
go test ./endpoints/health -run 'Manager|Register|Concurrent' -count=100
```

**Required commit:**

```text
feat(health): add validated manager construction
```

---

## [ ] API-008: Improve rate-limit decisions and bound cleanup work

**Priority:** P1
**Owner:** Runtime API team
**Size:** XL
**Depends on:** CI-004

### Work

Add a complete external-limiter decision type:

```go
type Decision struct {
    Allowed    bool
    Limit      int
    Remaining  int
    Reset      time.Time
    RetryAfter time.Duration
}

type DecisionLimiter interface {
    Allow(
        ctx context.Context,
        key string,
    ) (Decision, error)
}
```

Preserve the existing v4 `Limiter` through an adapter.

Ensure standard headers can be populated consistently for both in-memory and
external limiters.

Replace full-map cleanup under one global request-path mutex.

Acceptable designs include:

* Sharded maps with bounded shard cleanup.
* Expiration heap with generation tokens.
* Bounded incremental cleanup.
* Explicit background cleanup with a documented shutdown lifecycle.

Requirements:

* Cleanup work per request is bounded.
* High-cardinality state cannot cause an unbounded global scan.
* External limiter use remains recommended for distributed enforcement.
* Empty keys have an explicit policy.
* Trusted-proxy identity behavior is documented.
* Bypass headers remain disabled by default.

### Done when

* External limiters can supply complete rate-limit headers.
* Cleanup work is bounded and benchmarked.
* Race tests pass under high-cardinality concurrency.
* State expiry remains correct.
* No background goroutine leaks.
* The in-memory scale boundary is documented.

### Verification

```sh
go test ./middleware/ratelimit -race
go test ./middleware/ratelimit -run 'Decision|Cleanup|Concurrent' -count=100
go test ./middleware/ratelimit -run '^$' -bench Benchmark -benchmem
```

**Required commit:**

```text
feat(ratelimit): improve decisions and bounded cleanup
```

---

## [ ] API-009: Make buffered hard timeout behavior route-explicit

**Priority:** P1
**Owner:** Runtime API team
**Size:** L
**Depends on:** CI-004, PERF-002

### Work

For v4:

* Deprecate generic global use of `HardTimeout.Middleware()`.
* Keep cooperative timeout propagation as the default global middleware.
* Update all generators and examples to use cooperative timeout globally.
* Require explicit route-level wrapping for buffered hard timeout.
* Add route-capability validation for:

  * Streaming.
  * Server-sent events.
  * WebSocket upgrade.
  * Large downloads.
  * `http.Flusher`.
  * `http.Hijacker`.
  * `http.Pusher`.
  * `io.ReaderFrom`.
* Document the per-request goroutine and buffer cost.
* Add a lower-level event for a handler that continues running after timeout.

For v5:

* Keep cooperative timeout in the core module.
* Move buffered hard timeout to an optional runtime or contrib module.
* Remove the generic middleware alias from the stable core.

### Done when

* Generated services do not globally buffer all responses.
* Streaming routes preserve their writer capabilities.
* Finite JSON routes can explicitly opt into hard timeout.
* Package docs state allocation and goroutine behavior.
* Tests cover every optional writer interface relevant to supported platforms.

### Verification

```sh
go test ./middleware/timeout -race
go test ./middleware/timeout -run 'Streaming|Flusher|Hijacker|Pusher' -count=100
```

**Required commit:**

```text
fix(timeout): make buffered hard timeouts route explicit
```

---

# [ ] EPIC TST: Real adapter and integration evidence

**Goal:** Replace indirect or fake-only confidence with direct real-service
contracts for supported adapters.

**Epic completion criteria:**

* Supported PostgreSQL adapters pass real database contracts.
* Supported Redis adapters pass real Redis contracts.
* Provider integrations have recent sanitized sandbox evidence.
* Generated full-service integration is release-blocking.
* Adapter maturity classifications match actual evidence.

## [ ] TST-001: Add a reusable real PostgreSQL contract harness

**Priority:** P1
**Owner:** Integration test team
**Size:** L
**Depends on:** CI-001

### Work

Add a reusable internal harness under a path such as:

```text
contrib/internal/testpostgres
```

The harness must provide:

* Ephemeral database creation.
* Per-test schema isolation.
* Migration application.
* Transaction cleanup.
* Deterministic test data.
* Connection interruption support.
* Context cancellation support.
* Parallel-test safety.
* Sanitized logs.
* Version detection.
* A local `make test-postgres` target.
* A GitHub Actions service-container job.

Test against the PostgreSQL major versions declared in the adapter support
policy.

### Done when

* Harness tests fail when PostgreSQL is unavailable.
* Tests cannot accidentally use a developer's production DSN.
* Parallel packages do not share mutable schemas.
* Credentials never appear in logs or artifacts.
* The harness works locally and in CI.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make test-postgres
```

**Required commit:**

```text
test(postgres): add real adapter contract harness
```

---

## [ ] TST-002: Validate every supported PostgreSQL adapter against real PostgreSQL

**Priority:** P1
**Owner:** Adapter team
**Size:** XL
**Depends on:** TST-001

### Work

Apply the real contract harness to:

* `adapters/auditpostgres`
* `adapters/operationpostgres`
* `adapters/outboxpostgres`
* `adapters/webhookdeliverypostgres`
* `adapters/pgxpool`
* `adapters/txpostgres`
* Migration packages.
* PostgreSQL scheduler storage.
* Generated reference-service persistence paths.

Cover:

* Successful operations.
* Transaction rollback.
* Context cancellation.
* Connection loss.
* Duplicate keys.
* Serialization or concurrency conflict where applicable.
* Malformed schema.
* Migration mismatch.
* Resource cleanup.
* Nil and empty results.
* Multiple rows.
* Partial failure.
* Concurrent consumers.
* Restart and replay behavior for durable workflows.

Update:

* `docs/supported-adapter-test-realism.tsv`
* `docs/supported-adapter-contracts.tsv`
* Adapter maturity classifications.

### Done when

* No supported PostgreSQL adapter is classified using only fake-pool evidence.
* Real tests run on every PR touching a supported PostgreSQL adapter.
* Release evidence includes the real PostgreSQL job.
* Failure artifacts contain no credentials or customer-like data.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make test-postgres
GOWORK=off GOTOOLCHAIN=local make supported-adapter-check
```

**Required commit:**

```text
test(postgres): validate supported adapters on real postgres
```

---

## [ ] TST-003: Validate supported Redis adapters against real Redis

**Priority:** P1
**Owner:** Adapter team
**Size:** L
**Depends on:** CI-001

### Work

Keep miniredis for fast unit tests, but add a real Redis contract job for:

* Cache.
* Idempotency.
* Rate limiting.
* Any generated service path using Redis.

Cover:

* TTL expiration.
* Atomic reservation.
* Token mismatch.
* Concurrent retries.
* Script execution.
* Connection interruption.
* Context cancellation.
* Reconnect.
* Malformed stored data.
* Key isolation.
* Tenant isolation.
* Empty and oversized values.
* Server restart where practical.

### Done when

* Real Redis semantics are release evidence.
* Miniredis is not presented as equivalent to real Redis.
* Atomic operations are verified under concurrency.
* Redis errors follow documented fail-open or fail-closed policies.
* Logs redact connection credentials.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make test-redis
GOWORK=off GOTOOLCHAIN=local make supported-adapter-check
```

**Required commit:**

```text
test(redis): validate supported adapters on real redis
```

---

## [ ] TST-004: Require provider and generated-service integration evidence

**Priority:** P1
**Owner:** Integration test team
**Size:** XL
**Depends on:** TST-002, TST-003, CLI-004

### Work

Add two evidence levels.

### Required release-blocking generated integration

Generate a clean service and verify:

* Build.
* Unit tests.
* Migrations.
* PostgreSQL.
* Redis.
* Authentication wiring.
* Idempotency.
* Outbox and worker behavior.
* Health and admin endpoint separation.
* Graceful shutdown.
* Client generation.
* Upgrade from the previous supported release.

### Scheduled provider sandbox evidence

When protected sandbox credentials exist, test:

* Stripe.
* Resend.
* Clerk or equivalent provider path.
* OIDC discovery and JWK rotation where a sandbox is available.

Provider tests must:

* Use test or sandbox accounts.
* Create uniquely prefixed resources.
* Clean up created resources.
* Redact credentials and customer data.
* Publish only sanitized status.
* Record the last successful run date.

Require a recent successful provider run before promoting or retaining
`supported-adapter` status. Define the maximum acceptable evidence age in the
support policy.

### Done when

* Generated full-service integration blocks releases.
* Provider evidence age is visible.
* Missing provider secrets skip safely without implying success.
* A supported provider with stale evidence is downgraded or explicitly
  dispositioned.
* Generated integration runs outside the root workspace.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make generated-integration-check
GOWORK=off GOTOOLCHAIN=local make provider-live-check
```

**Required commit:**

```text
ci(integration): require provider and scaffold evidence
```

---

# [ ] EPIC PERF: Performance and resource management

**Goal:** Define scale boundaries, detect regressions, and prove that concurrency
features do not leak resources.

**Epic completion criteria:**

* Performance-sensitive changes have objective regression evidence.
* Hard timeouts do not leak goroutines.
* Idempotency and rate limiting behave correctly under concurrency.
* Reference-service load evidence is repeatable.

## [ ] PERF-001: Add controlled benchmark regression checks

**Priority:** P1
**Owner:** Performance engineer
**Size:** L
**Depends on:** CI-001, REL-004

### Work

* Run critical benchmarks multiple times.
* Compare with `benchstat` or an equivalent statistical comparison.
* Record machine metadata.
* Keep GitHub-hosted benchmark results advisory when variance is too high.
* Run blocking release benchmarks on a controlled runner.
* Define per-benchmark budgets for:

  * Allocations.
  * Bytes.
  * Meaningful latency regression.
* Require a PR performance note for budget changes.
* Store raw benchmark output as an artifact.
* Do not automatically update baselines from the same PR that exceeds them.

### Done when

* A known allocation regression is detected.
* Baseline updates require independent review.
* Results include commit, Go version, CPU, OS, and benchmark flags.
* Critical benchmark inventory matches `docs/performance.md`.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make benchmark-check
```

**Required commit:**

```text
ci(bench): add controlled performance regression checks
```

---

## [ ] PERF-002: Add hard-timeout stress and leak coverage

**Priority:** P1
**Owner:** Runtime test team
**Size:** L
**Depends on:** API-009

### Work

Stress hard timeout with:

* High request concurrency.
* Handlers that observe cancellation.
* Handlers that ignore cancellation.
* Handler panic before timeout.
* Handler panic after timeout.
* Response overflow.
* Late writes.
* Client cancellation.
* Slow writer.
* Partial writer failure.
* Repeated middleware construction.
* Repeated server shutdown.

Measure:

* Goroutine count recovery after requests complete.
* Allocation budget.
* Peak captured memory.
* Hook execution.
* Time to release buffered bodies.
* Absence of data races.

### Done when

* No unbounded goroutine growth is observed.
* Buffered body memory respects configured limits.
* Event hooks cannot crash request processing.
* Ignored cancellation is visible to operators.
* Stress tests pass repeatedly under the race detector.

### Verification

```sh
go test ./middleware/timeout -race -run Stress -count=20
go test ./middleware/timeout -run '^$' -bench Benchmark -benchmem
```

**Required commit:**

```text
test(timeout): add stress and leak coverage
```

---

## [ ] PERF-003: Add idempotency and rate-limit concurrency stress tests

**Priority:** P1
**Owner:** Runtime test team
**Size:** XL
**Depends on:** API-006, API-008, TST-003

### Work

For idempotency, test:

* Hundreds of callers using the same key.
* Different payload with same key.
* Reservation loss.
* Token mismatch.
* Store outage.
* Fail-open and fail-closed behavior.
* Cancellation during reservation.
* Cancellation during save.
* Ambiguous completion.
* Async telemetry queue saturation.
* Shutdown.
* Cross-tenant key isolation.

For rate limiting, test:

* High key cardinality.
* Hot key contention.
* State expiration.
* External limiter error.
* Fail-open and fail-closed behavior.
* Trusted proxy changes.
* Cleanup under sustained load.
* Header correctness.
* No unbounded map growth after TTL.

### Done when

* Race detector passes.
* Same-key idempotency executes the handler according to the documented
  contract.
* Rate-limit state returns to bounded size after expiry.
* Queue saturation does not block requests unexpectedly.
* No raw key or client secret appears in telemetry.

### Verification

```sh
go test ./middleware/idempotency ./middleware/ratelimit \
  -race \
  -run 'Stress|Concurrent' \
  -count=20
```

**Required commit:**

```text
test(runtime): add idempotency and rate limit stress tests
```

---

## [ ] PERF-004: Automate reference-service load regression evidence

**Priority:** P1
**Owner:** Performance engineer
**Size:** L
**Depends on:** TST-004, PERF-001

### Work

Run the reference service on a controlled runner and record:

* Request count.
* Concurrency.
* Throughput.
* p50, p95, and p99 latency.
* Timeout rate.
* Rate-limit rate.
* Unexpected status count.
* Heap delta.
* Total allocations.
* Allocations per request.
* Goroutine peak.
* Secret-leak scan result.
* Graceful shutdown duration.

Define regression budgets for stable routes. Keep them as release-review limits,
not public SLAs.

Publish:

* Raw JSON.
* Human summary.
* Baseline diff.
* Commit and environment metadata.

### Done when

* A known latency or allocation regression fails the controlled job.
* Load evidence is attached to every release candidate.
* Results are comparable because environment metadata is fixed.
* Reference-service results are not represented as universal application
  performance promises.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-load
GOWORK=off GOTOOLCHAIN=local make reference-service-load-check
```

**Required commit:**

```text
perf(reference): automate load regression evidence
```

---

# [ ] EPIC ARC: Scope, modules, and stable API boundaries

**Goal:** Turn the repository from a broad umbrella into a focused core plus
independently maintainable optional modules.

**Epic completion criteria:**

* The canonical core is small and defensible.
* CLI, adapters, and broad runtime concerns have independent dependency and
  release boundaries.
* V5 removes compatibility residue rather than adding more features.
* Every stable module has a compatibility gate.

## [ ] ARC-001: Freeze root API expansion and run an external stable-core review

**Priority:** P0
**Owner:** Lead architect
**Size:** M
**Depends on:** PRG-001

### Work

* Declare a temporary freeze on new stable root packages and exports.
* Open a public design issue for the stable-core review.
* Keep the issue open for at least seven calendar days.
* Invite review from:

  * Existing adopters.
  * Go library maintainers.
  * Security reviewers.
  * Developers who chose an alternative.
* Evaluate every current stable package against:

  * Root dependency necessity.
  * Generic reuse.
  * API stability.
  * Number of independent implementations.
  * Operational complexity.
  * Security sensitivity.
  * Whether application code should own the abstraction.
* Record disposition:

  * Retain in core.
  * Keep stable for v4 only.
  * Move to optional module.
  * Move to CLI.
  * Move to application code.
  * Remove in v5.

### Done when

* Every current stable package has a disposition.
* No new root API was added during the review without an emergency exception.
* External review comments and maintainer responses are recorded.
* The decision is reflected in package classification files.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make api-additions-check
```

**Required commit:**

```text
docs(api): freeze stable core for v5 review
```

---

## [ ] ARC-002: Define the focused v5 core surface

**Priority:** P1
**Owner:** Lead architect
**Size:** L
**Depends on:** ARC-001

### Work

Define a canonical v5 core with a target of approximately 8 to 12 primary
packages.

Default recommendation:

* `httpx`
* `fielderrors`
* `binding`
* `negotiation`
* `queryparams`
* `upload`
* `middleware/json`
* `middleware/maxbody`
* `middleware/querylimits`
* `middleware/secure`
* `routecontracts`
* A narrowly justified health package

Packages outside this set require explicit evidence to remain in the root.

Document:

* Core charter.
* Root dependency policy.
* Public API addition policy.
* Interface ownership policy.
* Context and cancellation policy.
* Error policy.
* Zero-value policy.
* Compatibility policy.
* Non-goals.

### Done when

* Root v5 has one coherent adoption story.
* Compatibility-only packages are not presented as recommended core.
* Every retained package has at least two credible reuse cases or a strong
  standardization reason.
* Every removed package has a migration destination.
* The README and API reference lead with the canonical core.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make package-classification-check
```

**Required commit:**

```text
docs(api): define focused v5 core surface
```

---

## [ ] ARC-003: Approve the module decomposition ADR

**Priority:** P1
**Owner:** Lead architect
**Size:** L
**Depends on:** ARC-002

### Work

Create an ADR deciding the target module layout.

Recommended decomposition:

1. Core library:
   `github.com/aatuh/api-toolkit/v5`
2. CLI and generator module.
3. PostgreSQL adapter module.
4. Redis adapter module.
5. Provider adapter module.
6. Observability adapter module.
7. Optional runtime module for complex features such as:

   * Idempotency.
   * Scheduler.
   * Operations.
   * Buffered hard timeout.
8. Generated examples and reference applications as non-library modules.

The ADR must define:

* Import paths.
* Tag prefixes.
* Release cadence.
* Compatibility promise.
* Dependency direction.
* API-diff baseline.
* Security support.
* Ownership.
* Deprecation process.
* Migration process.
* Whether modules remain in the monorepo or move to separate repositories.

Default recommendation: retain a monorepo initially, but use independent Go
modules and independent tags.

### Done when

* No optional adapter module pulls unrelated provider dependencies.
* Core has no third-party runtime dependencies.
* Module tag rules are mechanically testable.
* Each module has an owner and support tier.
* The ADR includes rejected alternatives and migration risk.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
```

**Required commit:**

```text
docs(adr): approve module decomposition
```

---

## [ ] ARC-004: Split the CLI into an independent module

**Priority:** P1
**Owner:** CLI team
**Size:** XL
**Depends on:** ARC-003

### Work

Move CLI and generator code from contrib into its approved module.

Requirements:

* Independent `go.mod`.
* Independent tag prefix.
* Independent changelog.
* Independent release workflow.
* No import cycle with core or adapters.
* Explicit dependency on released core and adapter versions.
* CLI version command.
* Generated project records the CLI version and template schema.
* Existing invocation path receives a migration shim or clear deprecation.
* Generated examples use the new module path.

### Done when

* Installing the CLI does not install unrelated provider libraries into a core
  consumer's dependency graph.
* CLI releases can occur without releasing core.
* Core releases can occur without releasing the CLI.
* Existing v4 CLI users have a documented migration path.
* All generator tests pass outside the root workspace.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make cli-check
GOWORK=off GOTOOLCHAIN=local make generated-integration-check
```

**Required commit:**

```text
refactor(cli)!: split generator into independent module
```

---

## [ ] ARC-005: Split contrib dependencies by domain

**Priority:** P1
**Owner:** Adapter team
**Size:** XL
**Depends on:** ARC-003, TST-002, TST-003

### Work

Split the current broad contrib module into independent dependency domains.

At minimum, isolate:

* PostgreSQL.
* Redis.
* Authentication providers.
* Messaging and email providers.
* Billing providers.
* OpenTelemetry and metrics.
* Router and HTTP adapters.
* Testing support.

Requirements:

* Installing a PostgreSQL adapter must not pull Stripe.
* Installing a Redis adapter must not pull OpenTelemetry exporters.
* Installing JWT middleware must not pull database drivers.
* Stable or supported adapter modules have independent API gates.
* Every module has a dependency footprint report.
* Every module has a license report.
* Cross-module contracts live in the narrowest possible shared module.

### Done when

* Dependency graph comparisons show material reduction.
* Module boundaries prevent reverse imports into core.
* Each optional module can be built and tested independently.
* Migration documentation lists exact new import paths.
* Supported adapter maturity remains evidence-based after the split.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make dependency-boundary-check
GOWORK=off GOTOOLCHAIN=local make dependency-report
GOWORK=off GOTOOLCHAIN=local make module-check
```

**Required commit:**

```text
refactor(adapters)!: split contrib dependency domains
```

---

## [ ] ARC-006: Reduce the v5 root API and remove compatibility residue

**Priority:** P1
**Owner:** Core API team
**Size:** XL
**Depends on:** ARC-002, ARC-003, API-001 through API-009

### Work

Use v5 for deletion and simplification.

Remove from the root where approved:

* Compatibility-only packages.
* Deprecated aliases.
* Weak constructors.
* Root interfaces now owned by consuming packages.
* Void response writers.
* Legacy idempotency options.
* Generic hard-timeout middleware.
* Migration-shaped abstractions.
* Provider-shaped abstractions.
* CLI-specific exports.
* Adapters and provider dependencies.

Normalize:

* Constructor naming.
* Error-returning behavior.
* Context rules.
* Nil receiver behavior.
* Zero-value behavior.
* Options struct conventions.
* Interface ownership.
* Package naming.

Do not add unrelated features during this ticket.

### Done when

* V5 root matches the approved core charter.
* Every removed symbol has a migration entry.
* Every retained symbol has a package-level use case and tests.
* API inventory contains no accidental export.
* Stable root package count matches the approved target.
* Root remains free of third-party runtime dependencies.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make release-api-check
GOWORK=off GOTOOLCHAIN=local make api-inventory-check
GOWORK=off GOTOOLCHAIN=local make dependency-boundary-check
```

**Required commit:**

```text
feat(core)!: reduce v5 stable surface
```

---

## [ ] ARC-007: Automate v4 to v5 migration

**Priority:** P1
**Owner:** Developer experience team
**Size:** XL
**Depends on:** ARC-004, ARC-005, ARC-006

### Work

Add:

* Complete symbol and package migration table.
* Import rewrite command.
* Validation-only dry-run mode.
* Machine-readable migration report.
* Detection of unsupported patterns.
* Before-and-after examples.
* Workspace guidance.
* Generated-project migration guidance.
* Compatibility fixture upgrades.
* Clear rollback instructions.

The migration tool must:

* Never silently rewrite unsupported code.
* Produce deterministic changes.
* Preserve formatting.
* Refuse dirty worktrees unless explicitly overridden.
* Print every changed file.
* Support `--check` mode for CI.
* Avoid modifying vendor or generated files unless requested.

### Done when

* All downstream compatibility fixtures migrate.
* The tool is idempotent.
* Running migration twice produces no second diff.
* Unsupported cases produce actionable diagnostics.
* Migration examples compile without local replace directives.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make migration-v5-check
GOWORK=off GOTOOLCHAIN=local make downstream-compat-check
```

**Required commit:**

```text
feat(migrate): automate v4 to v5 adoption
```

---

## [ ] ARC-008: Enforce compatibility separately for every stable module

**Priority:** P1
**Owner:** Compatibility lead
**Size:** L
**Depends on:** ARC-004, ARC-005, ARC-006

### Work

For every stable module:

* Define a baseline tag.
* Run API-diff checks.
* Require release notes for behavioral changes.
* Require an RC for minor releases.
* Maintain package classification.
* Maintain deprecation inventory.
* Maintain dependency footprint.
* Maintain downstream compatibility fixtures.
* Fail closed when the baseline tag is unavailable.

Do not apply a fake stable guarantee to experimental modules.

### Done when

* Every stable module has its own compatibility gate.
* Contrib-style drift reporting is no longer the only protection for supported
  adapter modules.
* Release evidence clearly separates stable, supported, and experimental
  modules.
* A breaking API fixture fails the correct module gate.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make module-api-check
```

**Required commit:**

```text
ci(api): enforce compatibility for stable modules
```

---

# [ ] EPIC CLI: CLI and generated-project production readiness

**Goal:** Make generator behavior deterministic, safe, independently releasable,
and honest about generated-code ownership.

**Epic completion criteria:**

* Generation is atomic and deterministic.
* Filesystem boundaries are safe.
* Generation does not require uncontrolled network access.
* Generated projects contain machine-readable origin metadata.
* CLI releases are verifiable and cross-platform.

## [ ] CLI-001: Make generation deterministic and atomic

**Priority:** P1
**Owner:** CLI team
**Size:** L
**Depends on:** ARC-004

### Work

* Render into a temporary directory.
* Validate the full output before replacing the destination.
* Use atomic rename where supported.
* Define fallback behavior for platforms without atomic directory replacement.
* Sort map and file iteration.
* Remove nondeterministic timestamps.
* Normalize line endings.
* Use stable file permissions.
* Ensure repeated generation from identical inputs produces identical content.
* Clean temporary output after failure.
* Never leave a partially generated project.

### Done when

* Two identical generation runs have identical content hashes.
* A forced mid-generation failure leaves the destination unchanged.
* Windows and Unix behavior are tested.
* Temporary directories are removed.
* Existing-file policy is explicit.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make cli-determinism-check
```

**Required commit:**

```text
fix(cli): make generation deterministic and atomic
```

---

## [ ] CLI-002: Harden generator filesystem boundaries

**Priority:** P0
**Owner:** Security engineer
**Size:** L
**Depends on:** CLI-001

### Work

Reject:

* `..` path traversal.
* Absolute output paths where not explicitly permitted.
* Symlink escapes.
* Hard-link surprises.
* Named pipes.
* Device files.
* Socket paths.
* Unsafe file permissions.
* Existing-file replacement without an explicit flag.
* Destination paths outside the approved root.

Add safe overwrite modes:

* `--check`
* `--fail-if-exists`
* `--overwrite-generated`
* Explicit dangerous full overwrite if retained

Require a confirmation-free noninteractive behavior suitable for CI.

### Done when

* Path traversal tests fail safely.
* Symlink escape tests fail safely.
* Existing user-owned files are preserved.
* Generated files are distinguishable from user-owned files.
* No partial files remain after rejection.
* Security documentation covers local filesystem threats.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make cli-security-check
GOWORK=off GOTOOLCHAIN=local make gosec
```

**Required commit:**

```text
fix(cli): harden filesystem boundaries
```

---

## [ ] CLI-003: Make generation offline and reproducible by default

**Priority:** P1
**Owner:** CLI team
**Size:** L
**Depends on:** CLI-001

### Work

* Bundle or version templates with the CLI release.
* Do not download mutable templates by default.
* Do not use `@latest` in generated files.
* Pin generated module dependencies according to a reviewed manifest.
* Add explicit `--allow-network` for workflows that genuinely need network
  access.
* Record every selected version in generated metadata.
* Add an offline integration test.
* Verify generated dependency checksums.
* Document how dependency updates are applied.

### Done when

* A standard generation test passes without network access.
* Generated outputs identify all input versions.
* Re-running generation with the same CLI version produces equivalent output.
* Network access cannot occur accidentally.
* Mutable remote template state cannot change a released CLI's output.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make cli-offline-check
```

**Required commit:**

```text
fix(cli): make generation offline and reproducible
```

---

## [ ] CLI-004: Add generated-project contract metadata and upgrade policy

**Priority:** P1
**Owner:** Developer experience team
**Size:** L
**Depends on:** CLI-001, CLI-003

### Work

Add a machine-readable file such as:

```text
.api-toolkit-project.json
```

Record:

* CLI version.
* Template schema version.
* Selected profile.
* Selected modules.
* Selected providers.
* Generation time, if needed, outside deterministic content comparisons.
* Generated file inventory.
* User-owned file inventory.
* Supported upgrade path.
* Source module versions.

Add commands:

* `api-toolkit project inspect`
* `api-toolkit project check`
* `api-toolkit project diff`
* `api-toolkit project upgrade --check`

Define:

* Which files may be regenerated.
* Which files are permanently application-owned.
* Whether in-place upgrade is supported.
* How conflicts are handled.
* How migration failure is rolled back.
* When users must perform a manual migration.

### Done when

* Generated ownership is machine-readable.
* The CLI never silently overwrites app-owned code.
* Upgrade check reports exact changes.
* Previous supported generated projects have upgrade fixtures.
* Unsupported old schemas fail with actionable guidance.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make generated-upgrade-compat-check
```

**Required commit:**

```text
feat(cli): add generated project contract metadata
```

---

## [ ] CLI-005: Publish verifiable cross-platform CLI releases

**Priority:** P1
**Owner:** Release engineer
**Size:** L
**Depends on:** ARC-004, SEC-005

### Work

Publish CLI binaries for supported platforms, including at minimum:

* Linux amd64.
* Linux arm64.
* macOS arm64.
* Windows amd64.

For every binary publish:

* SHA-256 checksum.
* SBOM.
* Provenance attestation.
* Signature or certificate.
* Version output.
* License notices.
* Installation instructions.
* Verification instructions.

Add:

* Shell completions.
* Man page or equivalent command reference.
* `--version --json`.
* Exit-code documentation.
* Backward compatibility policy for CLI flags and generated schema.

### Done when

* Every binary verifies independently.
* `--version --json` contains tag and commit.
* Binaries execute on the supported platform matrix.
* CLI release cadence is independent from core.
* Release artifacts contain no embedded credentials or local paths.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make cli-release-check
```

**Required commit:**

```text
build(cli): publish verifiable cross-platform releases
```

---

# [ ] EPIC SEC: Security and supply-chain maturity

**Goal:** Make security controls enforceable, auditable, and consistent with the
repository's public claims.

**Epic completion criteria:**

* Branch protection and secret scanning are actually enabled.
* Dependency updates have a defined maintenance cadence.
* OpenSSF evidence is current.
* Release tags and assets are verifiable.
* Workflows are safe for untrusted pull requests.
* Threat models match the new architecture.

## [ ] SEC-001: Enforce and verify branch protection

**Priority:** P0
**Owner:** Repository administrator
**Size:** M
**Depends on:** CI-003, GOV-001

### Work

Configure the protected default branch to require:

* Pull requests.
* Required status checks from `docs/required-checks.json`.
* At least one non-author approval.
* CODEOWNERS approval where applicable.
* Dismissal of stale approvals after new commits.
* Resolution of review conversations.
* Linear history.
* No force push.
* No branch deletion.
* Administrator inclusion unless an emergency procedure is invoked.

Add an authenticated governance check that produces sanitized evidence without
exposing tokens or private settings.

Document the emergency bypass procedure and audit requirements.

### Done when

* The governance check proves the settings are active.
* A test branch cannot merge without required checks.
* A test branch cannot force push over the protected branch.
* Emergency bypass use requires a public post-incident record.
* Evidence is included in release review.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make github-governance-check
```

**Required commit:**

```text
docs(governance): record enforced branch protections
```

---

## [ ] SEC-002: Enable and verify secret scanning and push protection

**Priority:** P0
**Owner:** Security engineer
**Size:** M
**Depends on:** SEC-001

### Work

* Enable GitHub secret scanning.
* Enable push protection.
* Add a pinned secret-scanning tool to CI for patterns GitHub may not cover.
* Scan current history and release assets.
* Verify generated fixtures use placeholders.
* Add redaction tests for:

  * Logs.
  * Error messages.
  * Release evidence.
  * Provider fixtures.
  * Generated configuration.
* Define credential exposure response:

  * Revoke.
  * Rotate.
  * Remove current content.
  * Assess artifacts and logs.
  * Coordinate history rewrite only when necessary.
  * Publish advisory where appropriate.

### Done when

* A known test secret is blocked by push protection or CI.
* Repository history has a documented clean or dispositioned result.
* Release evidence contains no credentials.
* Generated `.env.example` files contain placeholders only.
* Security-control status is checked during release review.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make secret-scan
GOWORK=off GOTOOLCHAIN=local make release-evidence-secret-check
```

**Required commit:**

```text
ci(security): enforce secret scanning controls
```

---

## [ ] SEC-003: Establish a dependency maintenance cadence

**Priority:** P1
**Owner:** Dependency maintainer
**Size:** M
**Depends on:** CI-001

### Work

Define and enforce:

* Weekly Dependabot review.
* Critical security update response.
* High security update response.
* Normal direct dependency update cadence.
* GitHub Action SHA refresh cadence.
* Grouping rules for compatible dependency families.
* Explicit close or defer reason for every stale update.
* License review for new dependencies.
* Dependency footprint review for optional modules.
* Maximum age for unresolved update PRs.

Triage all currently open dependency PRs.

Do not enable blind automerge for security-sensitive libraries or major updates.

### Done when

* No dependency PR is stale without a recorded disposition.
* Critical and high vulnerability handling matches `SECURITY.md`.
* Module dependency reports are current.
* Action pins remain immutable SHAs.
* Update responsibilities are assigned by module.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make dependency-report
GOWORK=off GOTOOLCHAIN=local make vuln
GOWORK=off GOTOOLCHAIN=local make actions-audit
```

**Required commit:**

```text
chore(deps): establish dependency maintenance cadence
```

---

## [ ] SEC-004: Reach and publish strong OpenSSF evidence

**Priority:** P1
**Owner:** Security engineer
**Size:** L
**Depends on:** SEC-001, SEC-002, SEC-003, GOV-001

### Work

* Review the current OpenSSF Scorecard result.
* Create a ticket for every failed or weak check.
* Reach a target score of at least 9.0 unless a specific check is genuinely not
  applicable.
* Record accepted exceptions with technical rationale.
* Register the project with the OpenSSF Best Practices program.
* Reach the applicable passing level.
* Publish current links and status in security documentation.
* Do not claim a badge before the external project record exists.

### Done when

* Scorecard result is current and public.
* The target score is met or every remaining exception is independently
  reviewed.
* Best Practices status is externally verifiable.
* Release review records both results.
* Badge links resolve.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make openssf-check
```

**Required commit:**

```text
docs(security): publish openssf compliance evidence
```

---

## [ ] SEC-005: Strengthen release authenticity

**Priority:** P1
**Owner:** Release engineer
**Size:** L
**Depends on:** REL-002, ARC-004, ARC-005

### Work

For every stable module and CLI release:

* Create a signed annotated tag or another documented strong tag-authentication
  mechanism.
* Produce GitHub build provenance.
* Produce module or binary SBOMs.
* Produce dependency-license reports.
* Sign SBOM and binary checksum manifests.
* Verify assets after upload.
* Record workflow identity.
* Record exact commit.
* Record tag reachability.
* Publish one copy-paste verification procedure.

Add negative contract tests for:

* Wrong tag identity.
* Wrong repository identity.
* Wrong workflow identity.
* Modified asset.
* Expired or invalid certificate.
* Missing attestation.
* Missing SBOM.

### Done when

* A consumer can verify release authenticity without trusting README prose.
* Root, adapters, and CLI assets have separate manifests.
* Uploaded assets are downloaded and reverified before publication.
* Verification fails for tampered fixtures.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make release-authenticity-check
```

**Required commit:**

```text
ci(release): strengthen release authenticity
```

---

## [ ] SEC-006: Harden workflow trust boundaries

**Priority:** P0
**Owner:** Security engineer
**Size:** L
**Depends on:** CI-003

### Work

Audit every workflow for:

* Minimal permissions.
* SHA-pinned actions.
* Untrusted pull-request code.
* `pull_request_target`.
* `workflow_run`.
* Repository write tokens.
* Secret availability.
* Artifact poisoning.
* Cache poisoning.
* Shell injection.
* Ref injection.
* Mutable action inputs.
* Unsafe release conditions.
* Fork behavior.
* Retention of sensitive artifacts.

Add a workflow threat model.

Add dependency review for pull requests that change module files.

Do not run untrusted code with write tokens or release secrets.

### Done when

* Every workflow has explicit permissions.
* No untrusted PR can access release or provider secrets.
* Action references are immutable.
* Workflow security checks are release-blocking.
* Threat-model findings are closed or explicitly dispositioned.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make actions-audit
GOWORK=off GOTOOLCHAIN=local make workflow-security-check
```

**Required commit:**

```text
ci(security): harden workflow trust boundaries
```

---

## [ ] SEC-007: Refresh the architecture threat model

**Priority:** P1
**Owner:** Security engineer
**Size:** L
**Depends on:** ARC-003, CLI-002, API-009

### Work

Update the threat model for:

* Core HTTP parsing.
* Response writing.
* Problem Details.
* Proxy identity.
* Authentication and authorization.
* Tenant boundaries.
* Idempotency.
* Rate limiting.
* Webhook verification and delivery.
* Health and pprof exposure.
* PostgreSQL and Redis adapters.
* Provider integrations.
* CLI filesystem access.
* Generated secrets.
* Generated deployment assets.
* Release workflows.
* Module supply chain.

For every threat, record:

* Asset.
* Actor.
* Entry point.
* Trust boundary.
* Abuse case.
* Existing mitigation.
* Required test.
* Residual risk.
* Owner.

### Done when

* Every high-risk package links to a threat-model section.
* Every critical mitigation has a test or external control.
* Streaming and operator-only endpoints are represented.
* CLI path traversal and generated secret handling are represented.
* Residual risks are visible to adopters.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make security-review-check
```

**Required commit:**

```text
docs(security): refresh architecture threat model
```

---

# [ ] EPIC GOV: Governance, maintenance, and external adoption

**Goal:** Reduce single-maintainer risk, make reviews independent, and set honest
support boundaries.

**Epic completion criteria:**

* At least two active maintainers can release and handle security issues.
* Stable API and releases require independent review.
* Issue intake is actionable.
* Release candidates receive real external adoption feedback.
* Maintenance and archival triggers are explicit.

## [ ] GOV-001: Define maintainers, CODEOWNERS, and release roles

**Priority:** P0
**Owner:** Project lead
**Size:** L
**Depends on:** PRG-001

### Work

Add `MAINTAINERS.md` defining:

* Active maintainers.
* Module owners.
* Security contacts.
* Release managers.
* Backup release manager.
* Emeritus process.
* Removal process.
* Succession process.
* Conflict-of-interest handling.
* Minimum activity expectations.

Update `.github/CODEOWNERS` by module and risk area.

Required target:

* At least two maintainers with repository and release capability.
* At least two people with private security advisory access.
* At least one backup capable of verifying and publishing a release.

Do not mark this complete based only on aspirational names.

### Done when

* Two active people have accepted responsibility.
* Release access is tested using a dry run.
* Security advisory access is tested.
* CODEOWNERS matches module ownership.
* No critical area has only an unavailable owner.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make github-governance-check
```

**Required commit:**

```text
docs(governance): define maintainers and ownership
```

---

## [ ] GOV-002: Require independent release and stable-API approval

**Priority:** P0
**Owner:** Project lead
**Size:** M
**Depends on:** GOV-001, SEC-001

### Work

Require:

* Non-author approval for stable API changes.
* Security review for security-sensitive packages.
* Independent release review.
* Independent verification of release assets.
* Independent approval of baseline or threshold changes.
* No self-approval of branch-protection exceptions.

Update:

* Governance docs.
* PR template.
* Release checklist.
* CODEOWNERS.
* Branch rules.

### Done when

* One person cannot author, approve, and publish a stable release alone under
  normal operation.
* Emergency release procedure is documented and audited.
* Stable API additions require public design review.
* Release evidence records both preparer and verifier.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make github-governance-check
GOWORK=off GOTOOLCHAIN=local make release-review-check
```

**Required commit:**

```text
docs(release): require independent release approval
```

---

## [ ] GOV-003: Strengthen issue and pull-request intake

**Priority:** P1
**Owner:** Community maintainer
**Size:** M
**Depends on:** PRG-002

### Work

Ensure issue templates collect:

* Module and package.
* Library, adapter, or CLI path.
* Project version.
* Go version.
* OS and architecture.
* Minimal reproduction.
* Expected and observed behavior.
* Concurrency context.
* Security sensitivity.
* Compatibility impact.
* Logs with secrets removed.
* Support-tier awareness.

Ensure PRs declare:

* Ticket ID.
* API impact.
* Behavioral impact.
* Security impact.
* Performance impact.
* Dependency impact.
* Generated-output impact.
* Migration impact.
* Tests run.

Add automated template validation where practical.

### Done when

* Bug reports are reproducible without requesting secrets.
* API proposals identify affected public symbols.
* Adopter reviews identify real integration friction.
* Security reports are redirected privately.
* PRs cannot omit compatibility classification.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make github-governance-check
```

**Required commit:**

```text
chore(github): strengthen issue and pull request intake
```

---

## [ ] GOV-004: Create a release-candidate adopter program

**Priority:** P1
**Owner:** Community maintainer
**Size:** L
**Depends on:** CI-005, ARC-007

### Work

For minor and major releases:

* Publish an RC.
* Open a public adopter-validation issue.
* Provide migration and test instructions.
* Request feedback on:

  * Install.
  * API clarity.
  * Documentation.
  * Compatibility.
  * Performance.
  * Generated code.
  * Adapter behavior.
* Require at least two external or independently maintained downstream
  validations before final v5.
* Record all blockers and dispositions.
* Do not count repository-owned golden fixtures as external adopters.

### Done when

* Two independent consumers validate the v5 RC.
* Every reported blocker is fixed or explicitly accepted.
* Migration time and friction are recorded.
* Final release notes cite adopter evidence without exposing private details.

### Verification

Evidence is reviewed through the public adopter issue and downstream compatibility
jobs.

**Required commit:**

```text
docs(adoption): add release candidate adopter program
```

---

## [ ] GOV-005: Define bounded support, maintenance, and archival triggers

**Priority:** P1
**Owner:** Project lead
**Size:** M
**Depends on:** GOV-001, ARC-003

### Work

Define per module:

* Supported versions.
* Security backports.
* Routine maintenance expectations.
* Response targets.
* Provider evidence requirements.
* Minimum Go support.
* Platform support.
* Deprecation period.
* End-of-life process.
* Module archival process.

Define triggers for:

* Downgrading an adapter from supported to experimental.
* Deprecating an unmaintained module.
* Archiving a module.
* Freezing new features.
* Requiring another maintainer.
* Refusing new provider integrations.
* Splitting a high-burden module.

Explicitly state that:

* Generated code is application-owned.
* There is no 24/7 support.
* Provider business workflows remain application-owned.
* Streaming is not a general toolkit abstraction.
* Deployment examples are starters, not universal production guarantees.

### Done when

* Every module has a support tier.
* Unsupported modules cannot appear as production-ready in docs.
* Archival decisions have objective triggers.
* Support commitments fit the available maintainer capacity.
* The policy is linked from the README and security policy.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make docs-check
GOWORK=off GOTOOLCHAIN=local make package-classification-check
```

**Required commit:**

```text
docs(support): define maintenance and archival triggers
```

---

# [ ] EPIC FIN: Releases and final 9/10 verification

**Goal:** Ship the non-breaking v4 hardening, validate the v5 redesign, and
publish independent evidence that the project meets the 9/10 target.

**Epic completion criteria:**

* v4 users receive safe additive improvements.
* V5 RC receives external and independent validation.
* V5 final release is verifiable and migration-complete.
* Post-release behavior validates the maintenance claims.
* Final scores are supported by evidence, not intent.

## [ ] FIN-001: Publish the additive v4 hardening release

**Priority:** P1
**Owner:** Release engineer
**Size:** L
**Depends on:** API-001 through API-009, CI-001 through CI-005, TST-001 through
TST-004, DOC-001 through DOC-005, REL-005

### Work

Prepare a v4 minor release containing only compatible additions and fixes:

* Checked response writers.
* Safe validation errors.
* Presence-aware binding opt-in.
* Strong idempotency constructor.
* Grouped idempotency options.
* Validated health constructor.
* Improved rate-limit decisions and cleanup.
* Safer timeout guidance and defaults.
* Real adapter evidence.
* Expanded CI and platform support.

Publish an RC before the final minor release.

### Done when

* API-diff reports no unintended v4 breakage.
* Downstream compatibility corpus passes.
* Release candidate receives adopter review.
* Release assets verify.
* Migration notes cover every deprecation.
* Changelog distinguishes source-compatible behavior changes from new APIs.

### Verification

```sh
test -n "${VERIFIED_V4_BASE_REF:?set by REL-000}"
API_BASE_REF="$VERIFIED_V4_BASE_REF" \
  GOWORK=off \
  GOTOOLCHAIN=local \
  make release-check
```

**Required commit:**

```text
chore(release): prepare v4.1.0
```

---

## [ ] FIN-002: Publish `v5.0.0-rc.1`

**Priority:** P1
**Owner:** Release engineer
**Size:** L
**Depends on:** ARC-004 through ARC-008, CLI-001 through CLI-005,
SEC-001 through SEC-007, REL-000, FIN-001

### Work

The RC must include:

* Focused v5 core.
* Independent optional modules.
* Clean canonical APIs.
* Full migration guide.
* Automated migration checks.
* Current Go and platform matrix.
* Real adapter contracts.
* Verifiable CLI release.
* Exact release evidence.
* Signed and attested assets.
* Known limitations.
* Support-tier table.
* Clear v4 support, supersession, or withdrawal status from `REL-000`.

Do not use a v5 release candidate to avoid dispositioning the published v4
release line or providing a safe v4 path for existing users.

### Done when

* All stable modules have RC tags.
* Every RC tag is reachable from the protected default branch.
* Migration fixtures pass.
* External adopter issue is open.
* Release assets verify.
* The v4 release-identity decision and consumer guidance are public.
* No unresolved P0 issue remains.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make rc-release-check
```

**Required commit:**

```text
chore(release): prepare v5.0.0-rc.1
```

---

## [ ] FIN-003: Complete an independent production-readiness audit

**Priority:** P0
**Owner:** Independent reviewer
**Size:** XL
**Depends on:** FIN-002, GOV-004

### Work

The reviewer must independently inspect and, where possible, execute:

* Exact RC commit.
* Module boundaries.
* Public APIs.
* Error behavior.
* Concurrency behavior.
* Race tests.
* Fuzzing.
* Real database and Redis contracts.
* Platform matrix.
* Dependency graph.
* Vulnerability checks.
* Workflow security.
* Release authenticity.
* Documentation accuracy.
* Migration flow.
* CLI filesystem safety.
* Generated project behavior.
* Maintainer and support policy.

The reviewer must not be the principal author of the v5 changes.

Publish:

* Exact commit.
* Scope.
* Commands.
* Findings.
* Uninspected areas.
* Severity.
* Required fixes.
* Final recommendation.

### Done when

* Every blocker and high finding is closed.
* Medium findings are fixed or have a dated disposition.
* The audit commit and release candidate commit are recorded.
* The audit does not rely solely on checked-in claims.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make audit-check
GOWORK=off GOTOOLCHAIN=local make release-check
GOWORK=off GOTOOLCHAIN=local make external-audit-fixture-check
```

**Required commit:**

```text
docs(audit): publish independent v5 readiness review
```

---

## [ ] FIN-004: Publish `v5.0.0`

**Priority:** P0
**Owner:** Release engineer
**Size:** L
**Depends on:** FIN-003, GOV-002

### Work

Publish final stable releases for every approved module.

Before tagging:

* Re-run every required check from a clean worktree.
* Confirm RC feedback is dispositioned.
* Confirm API baselines.
* Confirm current documentation.
* Confirm migration fixtures.
* Confirm dependency reports.
* Confirm SBOMs.
* Confirm signatures.
* Confirm provenance.
* Confirm default-branch reachability.
* Confirm two-person release approval.
* Confirm no open blocker or high issue.

### Done when

* All final tags and assets verify.
* Root and optional module versions install without local replaces.
* V4 to v5 migration instructions compile.
* README identifies v5 as current.
* Support policy identifies v5 as supported.
* V4 support or end-of-life status is explicit.
* Public API inventory matches released modules.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make stable-release-check
```

**Required commit:**

```text
chore(release): prepare v5.0.0
```

---

## [ ] FIN-005: Complete 90-day post-release validation and score closure

**Priority:** P1
**Owner:** Project lead
**Size:** L
**Depends on:** FIN-004

### Work

At 30, 60, and 90 days, record:

* Open defects by severity.
* Security reports.
* Breaking-change reports.
* Migration failures.
* API confusion reports.
* Documentation failures.
* Adapter incidents.
* Dependency update latency.
* CI reliability.
* Flaky test rate.
* Release verification failures.
* Maintainer response times.
* External adoption evidence.
* Support burden by module.
* Modules that should be downgraded, split, or archived.

Re-score every rubric area with evidence.

Do not assign 9/10 merely because this backlog was completed. Require observed
post-release stability.

### Final 9/10 exit criteria

| Area                  | Required evidence                                                                                                              |
|-----------------------|--------------------------------------------------------------------------------------------------------------------------------|
| Dependency-worthiness | Focused stable modules, two Go versions, downstream corpus, exact release evidence, no unresolved blockers.                    |
| Code quality          | Checked response errors, safe validation, explicit lifecycle, bounded cleanup, clean v5 APIs.                                  |
| Test quality          | Real PostgreSQL and Redis contracts, race, fuzz, mutation, platform matrix, stress and load evidence.                          |
| Documentation         | Accurate current version, five-minute quickstart, complete package docs, current migration guide.                              |
| Open-source trust     | Protected branch, secret scanning, two maintainers, independent review, verifiable releases, strong OpenSSF evidence.          |
| Ecosystem fit         | Focused guardrail positioning, documented alternatives, materially smaller dependency graphs.                                  |
| Scope                 | Core, adapters, runtime modules, and CLI have separate ownership and compatibility promises.                                   |
| API design            | V5 public API contains no weak constructors, hidden response failures, ambiguous required semantics, or compatibility residue. |

### Done when

* Every epic in this backlog is `[x]`.
* No P0 or P1 ticket remains open.
* Independent audit recommends production dependency use.
* Two downstream adopters remain operational after 90 days.
* No undocumented v5 breaking behavior has been reported.
* The checked-in scorecard links every score to verifiable evidence.

### Verification

```sh
GOWORK=off GOTOOLCHAIN=local make final-scorecard-check
GOWORK=off GOTOOLCHAIN=local make release-check
```

**Required commit:**

```text
docs(roadmap): complete 90-day production scorecard
```

---

# Recommended execution order

## Phase 1: Immediate trust repair

Complete first:

* PRG-001
* PRG-002
* REL-000
* REL-001
* REL-002
* REL-003
* REL-004
* DOC-001
* DOC-005
* CI-001
* SEC-001
* SEC-002
* REL-005

Do not begin a broad adoption campaign before this phase is complete.

## Phase 2: Non-breaking v4 production hardening

Then complete:

* DOC-002 through DOC-004
* CI-002 through CI-005
* API-001 through API-009
* TST-001 through TST-004
* PERF-001 through PERF-004
* SEC-003 through SEC-007
* GOV-001 through GOV-005
* FIN-001

## Phase 3: V5 scope reduction and module split

Then complete:

* ARC-001 through ARC-008
* CLI-001 through CLI-005
* FIN-002

## Phase 4: Independent validation and final release

Complete last:

* FIN-003
* FIN-004
* FIN-005

# Non-negotiable constraints

The following shortcuts would prevent a credible 9/10 result:

* Do not treat checked-in evidence for another commit as release evidence.
* Do not leave the stable release outside the default-branch history.
* Do not claim platform or Go-version support that CI does not test.
* Do not classify fake database tests as real adapter evidence.
* Do not preserve every current root package merely to avoid a major release.
* Do not add more provider integrations before the module split.
* Do not let one person author, approve, and publish normal stable releases.
* Do not mark generated code as supported application architecture.
* Do not expose arbitrary internal errors in public responses.
* Do not use API-diff success as proof of behavioral compatibility.
* Do not assign final scores before the 90-day post-release evidence exists.
