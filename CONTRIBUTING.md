# Contributing

Audience: contributors and maintainers preparing changes to `api-toolkit`.

Keep changes small, reviewable, and tied to the package or document they affect.
The default branch is `master`; public releases are cut from tagged, clean
release evidence.

## Local setup

Prerequisites:

- Go 1.25.x.
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

## Adopter feedback

Use [.github/ISSUE_TEMPLATE/adopter_review.md](.github/ISSUE_TEMPLATE/adopter_review.md)
for public adopter reviews. The template asks for adoption path, API friction,
missing docs, migration pain, what worked, and the requested outcome.

If GitHub Discussions are enabled for longer Q&A, maintainers may redirect
conversation there, but actionable API, docs, compatibility, or migration
follow-up should be captured in an issue. Do not post secrets, tokens, private
URLs, customer data, proprietary schemas, or vulnerability details in public
feedback.

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
- close questions after the answer is documented or the conversation has moved
  to the appropriate discussion space,
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

Non-maintainer PRs require at least one approving review and CODEOWNERS review
where ownership applies. Maintainer direct pushes should still use the same
checklist, run the narrowest proving validation command, and confirm no local
scratch files, secrets, `.audits`, or `.trash` entries are staged before
release.
