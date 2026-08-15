# V4 release-identity incident

Audience: v4 consumers, release operators, and independent reviewers. This is
the canonical record for the safe action and immutable evidence while the
published v4 history is repaired.

**Status:** Resolved on 2026-08-15. The root-only `v4.0.1` tag is the sole
verified v4 compatibility and release-evidence baseline:
`VERIFIED_V4_BASE_REF=v4.0.1`.

**Owner:** Release engineer

**Independent technical reviewer:** Codex evidence review, reproduced from a
clean checkout on 2026-08-15. This is independent of the original tag author,
but does not create a second repository maintainer or release authority.

**Evidence review date:** 2026-08-15

## Consumer action

Use root `github.com/aatuh/api-toolkit/v4@v4.0.1` for the supported v4 root
baseline and for `API_BASE_REF`. Do not use `v4.0.0`, `contrib/v4.0.0`, or
`contrib/v4.0.1` for a new deployment or release baseline.

The published `contrib/v4.0.1` module requires root `v4.0.0` with a checksum
that does not match the Go module proxy's immutable `v4.0.0` content. Go
correctly rejects that dependency. Contrib consumers must stay on their current
known-good version until a newly published paired contrib repair release.

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

## Reconciliation record

### Timeline

* **2026-07-11:** the `v4.0.0` release summary recorded Git commit
  `3cfc8d…`, while the Go module proxy retained origin `24188f7…` for the same
  version.
* **2026-07-20:** root `v4.0.1` was published at `09e0117…`. Its Git tag,
  module-proxy origin, and release-summary commit agree, and the commit is
  reachable from `master`.
* **2026-08-15:** the technical review selected root-only `v4.0.1` as the
  verified baseline, withdrew the other affected tags, and published the
  consumer notices.
* **2026-08-15:** protected PR #74 merged the final disposition into
  `master` as `dc84948e1c8d7b84a5ff2fdc4535cd6f3b81da09`.

### Root cause and impact

The root cause is a release-identity control gap: the historical evidence did
not bind the release tag, commit, tree, default-branch reachability, and Go
module identity together. That gap permitted the Git and module-proxy views of
`v4.0.0` to diverge without a release stop. The available evidence does not
establish an actor, intent, or compromise; this record does not infer one.

The impact is limited but material: root `v4.0.0` and both affected contrib
versions cannot be used as new baselines, and the published contrib `v4.0.1`
cannot resolve its required root checksum. Existing tags and assets remain
available only as immutable audit evidence.

### Recovery and verified history inventory

The protected safety branch used for this reconciliation was created from
current `master`. `git merge-base --is-ancestor v4.0.1 origin/master` succeeds
and `git log origin/master..v4.0.1` is empty. Therefore no commit from the
selected verified baseline was absent from `master`; there was nothing to
merge, reapply, or supersede. No published tag was moved or rewritten.

Recovery is to retain root `v4.0.1` as the root-only baseline and issue a new
paired contrib repair tag only after clean protected release evidence. The
release engineer owns that repair and the continuing release-identity review.

## Immutable tag and module evidence

`Git tag target` is the peeled annotated-tag commit from the repository.
`Proxy origin` is the commit reported by `go list -m -json` for the published
module version. Checksums are Go module zip checksums (`h1:`).

| Published tag | Git tag target and tree | Reachable from `master` | Proxy origin and module checksum | Status for consumers |
| --- | --- | --- | --- | --- |
| `v4.0.0` | `3cfc8d44423029ec50516d6b857d938b75067737`<br>`01a8f3d686d1923eed53d037ffd190a85b844f66` | No | `24188f75f7c65d41498781d5b48479fe6c65871b`<br>`h1:XiQQ/RTgNuLECNOHjIIU4P40FghmlnGF+cIIH9uLH6o=` | **Withdrawn — do not use.** Git and proxy identities diverge. |
| `contrib/v4.0.0` | `352d6574552d1822f573b27807144bf5f29a4a1f`<br>`b9a5e73a4c44fe7d68dd096d5b0ddf3dd8c21847` | No | `352d6574552d1822f573b27807144bf5f29a4a1f`<br>`h1:ViK7ZmlUQmpXpKyt9mwTIfW3/8gHl9NKG+TOP8Y3BG4=` | **Withdrawn — do not use.** Paired root tag is untrustworthy. |
| `v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`1cb357946d8df655a5ab5b230ba27dfb6957da7c` | Yes | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`h1:3AdpOFygErDjGFlDABj3GPy7erpnf0eFlIpq6cvFS1M=` | **Verified supported root baseline.** Not a contrib baseline. |
| `contrib/v4.0.1` | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`1cb357946d8df655a5ab5b230ba27dfb6957da7c` | Yes | `09e0117828c960453e3fb4cd028a02bc3e56ff33`<br>`h1:jGLOzYRBsh6beYbyuTO0yAgOt81DCn/hjQKOQ3d8DZk=` | **Withdrawn — do not use.** Root dependency checksum fails. |

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

## Final disposition and recovery

| Tag | Consumer action | Final status |
| --- | --- | --- |
| `v4.0.0` | Do not use. | Withdrawn — do not use. |
| `contrib/v4.0.0` | Do not use. | Withdrawn — do not use. |
| `v4.0.1` | Use for root-only v4 consumers and `API_BASE_REF`. | Verified supported root baseline. |
| `contrib/v4.0.1` | Do not use; await a paired repair release. | Withdrawn — do not use. |

The technical review reproduced all tag, proxy, and release-asset evidence from
a clean checkout. It verified both complete release asset manifests. The root
`v4.0.1` Git tag, module-proxy origin, release-summary commit, and reachable
default-branch commit all agree, so it is the sole root baseline. The other
three tags do not meet that same identity or paired-module criterion.

The repair path leaves every existing tag untouched and publishes a new paired
contrib repair tag only after clean protected release evidence. Use `v4.0.2`
only if the repair is patch-compatible; do not create v5 merely to evade this
incident.

## Technical review record

The 2026-08-15 review independently completed:

1. Re-run the evidence commands below from a clean checkout and compare every
   tag target, tree, proxy origin, and checksum with this record.
2. Download each published release asset and verify its SHA-256 digest against
   the corresponding manifest above.
3. Confirm that published tags have not changed again and that the selected
   verified baseline is an ancestor of `origin/master`.
4. Publish the final status of all four affected tags.
5. Require the next paired root/contrib tag to have exact commit-bound evidence,
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
