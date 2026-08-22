# Contributing

Audience: contributors and maintainers preparing changes to `api-toolkit`.

## Current Release Identity

Contributions and release-related examples use the verified root baseline
`v4.0.1`. `v4.0.0`, `contrib/v4.0.0`, and `contrib/v4.0.1` are withdrawn; see
`docs/release-incident-v4-release-identity.md` before proposing release or
module-version changes.

Keep changes small, reviewable, and tied to the package or document they affect.
The default branch is `master`; public releases are cut from tagged, clean
release evidence.

## Local setup

Prerequisites:

- Go 1.25.x (minimum) or Go 1.26.x (current tested line).
- `make`.
- GitHub CLI only when running optional repository governance checks.

The repository contains two Go modules: root and `contrib`. The Makefile sets
`GOWORK=off` so parent workspaces do not affect local checks.

Useful commands:

```sh
GOTOOLCHAIN=local make docs-check
GOTOOLCHAIN=local make fast-check
GOTOOLCHAIN=local make audit-check
GOTOOLCHAIN=local make finalize
```

Use the narrowest command that proves the change. `make finalize` installs tools
and may rewrite Go formatting and module files, so run it only when that
mutation is intended.

## Tests

Add or update tests for behavior changes. Prefer package-level unit tests for
pure library behavior, contract tests for reusable interfaces, and opt-in
integration checks for provider or generated-service behavior.

For documentation and repository-policy changes, add or update `docscheck`
coverage when the rule should not regress.

## API compatibility

Stable core packages are listed in `VERSIONING.md` and mirrored by
`docs/package-classification.tsv` and `scripts/apicheck.sh`.

Rules:

- Breaking stable API changes require a major version.
- New stable packages or exported identifiers need docs, tests, compatibility
  review, and release-note consideration.
- New stable root packages or promotions require the stable API review board
  process in `docs/governance.md`: a public design issue, at least 7 calendar
  days for comment, and maintainer approval.
- No new `ports` export is accepted without the design evidence described in
  `docs/stable-core.md` and `docs/ports-surface.md`.
- Contrib supported-adapter incompatible drift is gate-enforced and does not
  make contrib part of the stable core API promise.

## Documentation policy

Update docs when behavior, API, compatibility, generated output, release
process, or production caveats change.

Use:

- `README.md` for the adoption path and high-level pointers,
- `docs/README.md` as the documentation index,
- `VERSIONING.md` for stable API policy,
- `docs/stable-core.md` for the small-core charter,
- `docs/alternatives.md` for ecosystem positioning,
- `docs/release-runbook.md` for release commands.

## Security reports

Do not open public issues for vulnerabilities. Use `SECURITY.md` and GitHub
private vulnerability reporting. Normal PRs must avoid leaking secrets, tokens,
PII, raw provider payloads, or internal parser/database errors.

## Questions And Discussions

GitHub Discussions are disabled for this repository. Use GitHub issues as the
public place for focused usage questions, API proposals, documentation gaps, and
adopter feedback so each answer can be searched, linked to a release, or turned
into an actionable follow-up.

For a usage question, start with
[.github/ISSUE_TEMPLATE/question.md](.github/ISSUE_TEMPLATE/question.md) and
link the documentation or example you tried. Use the bug, feature, API-change,
documentation, or adopter-review template when the question already identifies
a defect, a request, or adoption friction. Security reports do not belong in
issues; use `SECURITY.md` and GitHub private vulnerability reporting.

Questions are best effort, not a support-service SLA. Maintainers may close a
question after recording the answer in documentation, linking an existing
answer, or opening a scoped follow-up issue. Do not include secrets, tokens,
private URLs, customer data, proprietary schemas, or vulnerability details in a
public issue.

## Adopter feedback

Use [.github/ISSUE_TEMPLATE/adopter_review.md](.github/ISSUE_TEMPLATE/adopter_review.md)
for public adopter reviews. The template asks for adoption path, API friction,
missing docs, migration pain, what worked, and the requested outcome.

Keep actionable API, docs, compatibility, and migration follow-up in issues as
required by the Questions And Discussions policy. Do not post secrets, tokens,
private URLs, customer data, proprietary schemas, or vulnerability details in
public feedback.

## Issue Triage

This is a single-maintainer project, so public issue triage is best effort. Aim
to acknowledge, label, request the missing reproduction details, or explain the
next decision within 14 calendar days. Security reports are different: do not
triage them in public; use `SECURITY.md` and GitHub private vulnerability
reporting, which has its own three-business-day acknowledgement target.

At first review, classify the issue with the most useful available label:

- `good first issue` for a bounded task with enough context for a newcomer,
- `docs` for a documentation or example change,
- `api-review` for a stable root API proposal or compatibility decision,
- `security` only for public safety coordination that contains no vulnerability
  details; otherwise redirect privately,
- `breaking-change` when a reported behavior or proposal can affect a public
  source, runtime, configuration, or generated-output contract,
- `needs-design` when implementation should wait for a scoped design issue or
  adopter feedback.

Use the existing issue templates before asking reporters to rewrite a report.
For bugs, ask for the smallest safe reproduction, affected package or command,
api-toolkit and Go versions, operating-system context, expected behavior, and
observed behavior. Never request secrets, customer data, private URLs, raw
provider payloads, or vulnerability details in a public issue.

Close an issue only with a brief reason:

- close as fixed when the issue links to the merged pull request or released
  version,
- close duplicates with a link to the canonical issue,
- close out-of-scope proposals after explaining the applicable non-goal,
  app-owned alternative, contrib path, or design discussion,
- request missing reproduction details before closing an unreproducible bug;
  close it after 30 days without a response and invite the reporter to reopen
  with the requested safe details,
- close questions after the answer is documented, linked to an existing answer,
  or captured in a scoped follow-up issue,
- immediately redirect a public security report to the private channel and
  remove sensitive detail when repository administrators can do so safely.

Do not close an active `needs-design`, `api-review`, or adopter-feedback issue
solely because no implementation is scheduled. Mark it deferred with the next
review trigger instead.

## Conduct

Project spaces follow `CODE_OF_CONDUCT.md`. Keep disagreement technical,
evidence-based, and focused on the change under review.

## Pull request review

Every pull request should explain:

- what changed and why,
- tests run,
- documentation changes,
- compatibility impact,
- security impact,
- benchmark or performance impact.

## Backlog ticket and commit discipline

One backlog ticket per pull request is required. Keep unrelated work out of the
same pull request; split work that cannot be reviewed and released as one
ticket. The pull-request title must use conventional commit syntax, for example
`feat(binding): add presence-aware required fields`.

One final conventional commit per ticket is required. Before merge, make the
final commit body contain:

```text
Refs: <ticket-id>
```

Classify every change explicitly as one of: no public effect, additive API,
behavioral change, deprecation, or breaking change. Also record the security
classification, verification commands and results, documentation and release
note impact, generated-file impact, benchmark impact, and migration impact in
the pull request.

Breaking changes use `!` in the conventional-commit subject and include this
footer in the final commit body:

```text
BREAKING CHANGE: <migration impact and replacement>
```

### Final squash merge

Use **Squash merge** for backlog tickets. Before merging, set the squash commit
subject to the conventional pull-request title and preserve the `Refs:` footer
and any required `BREAKING CHANGE:` footer in the final commit body. This keeps
the protected default branch at one final conventional commit per ticket.

`master` changes require a pull request, resolved conversations, and passing
required checks. As a sole-maintainer repository, it requires zero approvals
until a second eligible maintainer is available; then independent and
CODEOWNERS review should be enabled. Every PR should use the same checklist, run the narrowest proving validation command, and confirm no local scratch files, secrets, `.audits`, or `.trash` entries are staged before release.
