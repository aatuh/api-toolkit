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

## Pull request review

Every pull request should explain:

- what changed and why,
- tests run,
- documentation changes,
- compatibility impact,
- security impact,
- benchmark or performance impact.

Non-maintainer PRs require review. Maintainer direct pushes should still use the
same checklist before release.
