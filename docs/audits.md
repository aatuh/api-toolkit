# Audit and Scratch Archive Policy

Audience: maintainers, release reviewers, and auditors checking which evidence
belongs in the public repository.

`.audits/` and `.trash/` are local-only scratch directories. They may exist in a
maintainer working tree for continuity while backlog remediation is in progress,
but they must not be tracked, linked as active documentation, or treated as
release evidence.

## Public Audit Evidence

Promote durable findings into tracked docs, release notes, release evidence
summaries, or checked-in backlog files. Raw audit transcripts, temporary
investigation notes, copied rendered pages, and discarded drafts should stay out
of Git unless an explicit maintainer decision promotes them.

Release publication evidence is owned by the release workflow, release runbook,
release manifests, and `release-check-summary.json`. Local dirty-tree audit
evidence is allowed only when documented by the release policy, and it is not release evidence for publication.

## Local Scratch Directories

The local `.audits/` directory is for private audit transcripts, working
backlogs, and reviewer notes that are useful while the current remediation loop
is active. Durable conclusions belong in tracked files such as
`docs/production-readiness.md`, `docs/release-review.md`, or specific package
guides.

The local `.trash/` directory is not active product documentation and is not
active product evidence. It is a maintainer scratch archive for discarded or
superseded material, and active docs should not link into `.trash/`.

If archived material needs to become public, move it into `docs/archive/` with
explicit status, date, owner, and superseding document links. Do not expose raw
scratch trees from the repository root.

## Verification

Before publishing or marking a hygiene ticket complete, verify the tracked tree
does not contain the scratch directories:

Use `git ls-files -- .audits .trash` for the current tracked tree and
`git log --all -- .audits .trash` for maintained local history before running
the docs gate.

```sh
git ls-files -- .audits .trash
git log --all -- .audits .trash
GOTOOLCHAIN=local make docs-check
```

The first two commands should print no paths in the maintained public history.
`make docs-check` includes a repository-hygiene contract that fails if
`.audits` or `.trash` re-enter the tracked tree.
