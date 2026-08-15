# V4 release-identity incident

Audience: v4 consumers, release operators, and independent reviewers. This is
the canonical record for the safe action and immutable evidence while the
published v4 history is repaired.

**Status:** Open — no v4 tag is approved as a new release-evidence baseline.
`VERIFIED_V4_BASE_REF` is deliberately unset.

**Owner:** Release engineer

**Independent reviewer:** Unassigned; required before a final disposition

**Evidence review date:** 2026-07-22

## Consumer action

Do not adopt `v4.0.1` or `contrib/v4.0.1` for a new deployment. Do not start a
new deployment with `v4.0.0` or `contrib/v4.0.0` either, and do not use any of
those tags as `API_BASE_REF`.

The published `contrib/v4.0.1` module requires root `v4.0.0` with a checksum
that does not match the Go module proxy's immutable `v4.0.0` content. Go
correctly rejects that dependency. Existing consumers should pin their current
known-good dependency versions and review this document before upgrading.

Published tags and release assets will not be moved, deleted, recreated, or
overwritten. A repair uses a new SemVer-correct tag after the final decision.

## Incident summary

The public root `v4.0.0` module identity and the currently advertised Git tag
are different commits:

* The Go module proxy resolves root `v4.0.0` to
  `24188f75f7c65d41498781d5b48479fe6c65871b`.
* The current Git tag `v4.0.0` resolves to
  `3cfc8d44423029ec50516d6b857d938b75067737`, whose parent is `24188f7…`.
* The `v4.0.0` release summary identifies `3cfc8d…` as its tested commit.

This proves that the tag identity used by the Go module ecosystem differs from
the tag identity currently available from Git. It establishes a release
integrity incident. It does not establish a cause, compromise, or attribution.
It also does not establish who changed the tag or why it changed.

The v4.0.0 and v4.0.1 release commits share common ancestor
`24c46a2efc50f8d03691047eb425122a740bd26a`; neither release commit is an
ancestor of the other. `v4.0.1` is reachable from `master`; `v4.0.0` is not.

## Immutable tag and module evidence

`Git tag target` is the peeled annotated-tag commit from the repository.
`Proxy origin` is the commit reported by `go list -m -json` for the published
module version. Checksums are Go module zip checksums (`h1:`).

| Published tag | Git tag target and tree | Reachable from `master` | Proxy origin and module checksum | Status for consumers |
| --- | --- | --- | --- | --- |
| `v4.0.0` | `3cfc8d44423029ec50516d6b857d938b75067737`<br>`01a8f3d686d1923eed53d037ffd190a85b844f66` | No | `24188f75f7c65d41498781d5b48479fe6c65871b`<br>`h1:XiQQ/RTgNuLECNOHjIIU4P40FghmlnGF+cIIH9uLH6o=` | Do not use; Git and proxy identities diverge. |
| `contrib/v4.0.0` | `352d6574552d1822f573b27807144bf5f29a4a1f`<br>`b9a5e73a4c44fe7d68dd096d5b0ddf3dd8c21847` | No | `352d6574552d1822f573b27807144bf5f29a4a1f`<br>`h1:ViK7ZmlUQmpXpKyt9mwTIfW3/8gHl9NKG+TOP8Y3BG4=` | Do not use; paired root tag is not trustworthy. |
| `v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`1cb357946d8df655a5ab5b230ba27dfb6957da7c` | Yes | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`h1:3AdpOFygErDjGFlDABj3GPy7erpnf0eFlIpq6cvFS1M=` | Do not use as a paired v4 baseline pending independent review. |
| `contrib/v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`1cb357946d8df655a5ab5b230ba27dfb6957da7c` | Yes | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`h1:jGLOzYRBsh6beYbyuTO0yAgOt81DCn/hjQKOQ3d8DZk=` | Do not use; its root dependency checksum fails. |

The published `contrib/v4.0.1` `go.sum` records:

```text
github.com/aatuh/api-toolkit/v4 v4.0.0 h1:6ObM4eLrw6Z4jITc1544E5BHJdLZ1l1Tu1O1MufP31o=
```

The proxy's root `v4.0.0` zip checksum is:

```text
h1:XiQQ/RTgNuLECNOHjIIU4P40FghmlnGF+cIIH9uLH6o=
```

The root `v4.0.0` `go.mod` checksum agrees in both records:

```text
h1:BAZGQkcxNfPRa12e5LdttmxC1MISTqiGtnqi5mRRDEs=
```

This narrows the failure to the root module archive identity, rather than its
`go.mod` file.

## Published release assets

There are GitHub releases for root tags only; no GitHub release exists for
`contrib/v4.0.0` or `contrib/v4.0.1`. The root release asset sets contain both
root and contrib SBOMs. The full per-asset digest manifest for each release is
published as an asset and is reproduced below for independent verification.

| Release | Published | Tested commit in `release-check-summary.json` | Manifest SHA-256 |
| --- | --- | --- | --- |
| [`v4.0.0`](https://github.com/aatuh/api-toolkit/releases/tag/v4.0.0) | 2026-07-11T14:34:38Z | `3cfc8d44423029ec50516d6b857d938b75067737` | `ed28b7a2a453540a5044134e8957a3eb72b19ba692871cea1f03f0022f32e90b` |
| [`v4.0.1`](https://github.com/aatuh/api-toolkit/releases/tag/v4.0.1) | 2026-07-20T12:08:15Z | `09e0117828c960453e3fb4cd028a02bc3e56ff33` | `65b7553016a7ba10194fa5c79f930ad8c527e7072fb9d3e4cae28771a665278f` |

`v4.0.0` asset digests:

```text
ac0ae06d4d2c57c127e8fb4ac36c76d31df6014840ce64e6b01158b6a37308b5  release-check-summary.json
58a8ed81a6941c8b9b85e30d4f696b4bba5c4c6ef5dc42e46a137a669593696d  release-evidence-logs.tgz
46e83ea60e476e2b7dc25c95709cfc6cbe8f05b1051aa7612c6b2b9dbd5abc5f  sbom-root.spdx.json
38972ce8f755f3f7b6452a04cbb2258c8317bdf7c9b8965052b5dee080ee26ce  sbom-contrib.spdx.json
8d1c3d03c7549c931f10f3caddbdff041d11fda0fb4761e2e37c4637543175aa  dependency-licenses-root.tsv
dcb1d645b743e7d4ccd1583cf192576fddf490ff8c8d8e5a7e41535b0ba98c31  dependency-licenses-contrib.tsv
ae8f368c06f6d5e5a6724d11a00efa31c065efe23ce72c352d9914c10ddaa20e  sbom-root.spdx.json.sig
bfc20a0c070da3a9de181278ab90f666d046397d596ae24c5be3d5f08755c5c4  sbom-root.spdx.json.pem
44259d2944a99ab6c8732628eca51765f9380009ed7855229a9b21ce4c2fb6fb  sbom-contrib.spdx.json.sig
39c38bb119ca0cbe27a8fe1f547dd346a71b2b8cfa3ec1431d1923f279654bb4  sbom-contrib.spdx.json.pem
```

`v4.0.1` asset digests:

```text
fe1855ba31e54a1661604853a9ead91a1d00c8be658671013563e44bc98584bc  release-check-summary.json
d2f816fd29c516fddf1dda06ae3c9943c0d0c3248cb9f9c6f214042c9bfa4f49  release-evidence-logs.tgz
c4a07efe96bbb58e549e7a4c6dc56affb15a2824b9b30d25502034676f6bb4bd  sbom-root.spdx.json
bf9bb33e3a0c405d39e24f5b40e970372ed0b73f2baf5c50046ce7d20508e7c5  sbom-contrib.spdx.json
8d1c3d03c7549c931f10f3caddbdff041d11fda0fb4761e2e37c4637543175aa  dependency-licenses-root.tsv
ab6daf232d07a0c4509e8434f53859a864404f5710b475180a7cf6f985f3b2d1  dependency-licenses-contrib.tsv
dde0efe3d3de1960cb22f393a756328758b5b225c39988f7b072957aa5d47502  sbom-root.spdx.json.sig
b546dc80e2c56de085f289b9eeafa20b89ccac734f506c5baae6d67d1f049e41  sbom-root.spdx.json.pem
d3d4bbf3b49d8cfe7a9acf737812638e8fa2e481d8113b00ee4700989da23fe2  sbom-contrib.spdx.json.sig
04aa97595b84af81f36079c48b4c5d9c48a0b4614ceebc2c4cbaf9c9fb3a6995  sbom-contrib.spdx.json.pem
```

The asset manifests and the GitHub asset API supply these SHA-256 values. Their
presence does not make either release verified: the v4.0.0 summary schema does
not bind a release tag, commit tree, branch reachability, or module identities
to the evidence. That missing binding is remediated by REL-002.

## Provisional disposition and recovery

The following is a safety hold, not the final public status required by
REL-000:

| Tag | Provisional safety hold | Final status |
| --- | --- | --- |
| `v4.0.0` | Do not use: Git tag and proxy identity diverge. | Pending independent review. |
| `contrib/v4.0.0` | Do not use: paired root release is untrustworthy. | Pending independent review. |
| `v4.0.1` | Do not use as a paired release baseline. | Pending independent review. |
| `contrib/v4.0.1` | Do not use: root dependency checksum fails. | Pending independent review. |

The release owner and independent reviewer must make one final public choice
for each tag: **Verified supported baseline**, **Superseded by a verified
release**, or **Withdrawn — do not use**. They must publish that same choice in
the GitHub release notes, `CHANGELOG.md`, README, support policy, and release
runbook.

The proposed repair path is to leave every existing tag untouched, reconcile
the chosen verified code history into `master`, and publish a new paired v4
repair tag. Use `v4.0.2` only if the repair is patch-compatible; do not create
v5 merely to evade this incident. The reviewer must approve the SemVer choice
and record the eventual `REPAIR_RELEASE_TAG` and `VERIFIED_V4_BASE_REF` here.

## Independent review checklist

Before setting `VERIFIED_V4_BASE_REF` or publishing a repair release, the
reviewer must independently:

1. Re-run the evidence commands below from a clean checkout and compare every
   tag target, tree, proxy origin, and checksum with this record.
2. Download each published release asset and verify its SHA-256 digest against
   the corresponding manifest above.
3. Confirm that published tags have not changed again and that the selected
   verified baseline is an ancestor of `origin/master`.
4. Decide and publish the final status of all four affected tags.
5. Confirm the next paired root/contrib tag has exact commit-bound evidence,
   asset verification, and no `replace`-based module installation.

## Evidence commands

Run these commands from a clean checkout. They are evidence collection, not a
substitute for the independent review above.

```sh
git show-ref --tags -d | rg '(^|/)(v4\.0\.[01]|contrib/v4\.0\.[01])($|\^\{\})'
git rev-parse v4.0.0^{} v4.0.0^{}^{tree}
git rev-parse contrib/v4.0.0^{} contrib/v4.0.0^{}^{tree}
git rev-parse v4.0.1^{} v4.0.1^{}^{tree}
git rev-parse contrib/v4.0.1^{} contrib/v4.0.1^{}^{tree}
git merge-base v4.0.0 v4.0.1
git merge-base --is-ancestor v4.0.1 master
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/v4@v4.0.0
GOWORK=off GOTOOLCHAIN=local go list -m -json github.com/aatuh/api-toolkit/contrib/v4@v4.0.1
GOWORK=off GOTOOLCHAIN=local make docs-check
```
