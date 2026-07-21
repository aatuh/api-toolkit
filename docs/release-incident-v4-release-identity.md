# V4 release-identity incident

Audience: v4 consumers, release operators, and reviewers who need the current
safe action while the repository reconciles its published v4 tag history.

**Status:** Open — no v4 tag is approved as a new release-evidence baseline.

## Consumer guidance

Do not adopt `v4.0.1` or `contrib/v4.0.1` for a new deployment, and do not use
either tag as `API_BASE_REF`. The published contrib module records a checksum
for root `v4.0.0` that differs from the checksum returned by the Go module
proxy. Go correctly rejects that dependency with a checksum mismatch.

Existing v4 consumers should pin their current known-good dependency versions
and review this incident before upgrading. A repair release will use a new tag;
published tags will not be moved, recreated, or reused.

## What is known

| Tag | Immutable commit | Current status |
| --- | --- | --- |
| `v4.0.0` | `3cfc8d44423029ec50516d6b857d938b75067737` | Published; branch reachability and release evidence are under review. |
| `contrib/v4.0.0` | `352d6574552d1822f573b27807144bf5f29a4a1f` | Published; paired release evidence is under review. |
| `v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33` | Published; not approved as a release-evidence baseline. |
| `contrib/v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33` | Published; do not use pending a replacement release. |

The v4.0.0 and v4.0.1 release commits have a common ancestor but neither is
an ancestor of the other. `v4.0.1` is reachable from `master`; `v4.0.0` is
not.

The checksum mismatch is specifically between the `contrib/v4.0.1` record for
`github.com/aatuh/api-toolkit/v4 v4.0.0` and the Go module proxy result:

```text
recorded: h1:6ObM4eLrw6Z4jITc1544E5BHJdLZ1l1Tu1O1MufP31o=
fetched:  h1:XiQQ/RTgNuLECNOHjIIU4P40FghmlnGF+cIIH9uLH6o=
```

This evidence establishes an integrity problem in the published dependency
pair. It does not establish a cause, compromise, or attribution.

## Recovery criteria

Before a new v4 release is published, the release operator and an independent
reviewer must:

1. Preserve and record the immutable tag, tree, module-proxy, asset, and
   branch-reachability evidence for every affected tag.
2. Document the cause and consumer impact of the divergent histories and the
   checksum mismatch.
3. Select one verified v4 compatibility baseline and record it as
   `VERIFIED_V4_BASE_REF` in the release runbook and release evidence.
4. Publish a new SemVer-correct repair tag. `v4.0.1` must not be reused.
5. Update GitHub release notes, this incident, the README, support policy, and
   release runbook with the final status of each affected tag.

Until those criteria are met, release evidence must fail closed rather than
silently using the latest published v4 tag.

## Evidence commands

Run these commands from a clean checkout when reviewing the incident:

```sh
git show-ref --tags -d | rg 'v4\.0\.[01]'
git merge-base v4.0.0 v4.0.1
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/v4@v4.0.1
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/contrib/v4@v4.0.1
GOWORK=off GOTOOLCHAIN=local make docs-check
```

The `go list -m` commands prove module discovery and origin metadata only.
They do not replace asset, provenance, or release-evidence verification.
