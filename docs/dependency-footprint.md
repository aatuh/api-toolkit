# Dependency Footprint

Audience: adopters and release reviewers deciding whether the dependency graph is
appropriate for their service.

Run:

```sh
GOTOOLCHAIN=local make dependency-report
```

The report writes review artifacts under `.ci-result/dependencies/`:

| Artifact | Purpose |
| --- | --- |
| `summary.md` | Human-readable root/contrib dependency summary and optional base-ref diff counts. |
| `summary.tsv` | Machine-readable direct require, indirect require, and build-list counts. |
| `root.modules` | Current root module build-list dependencies from `go list -m all`. |
| `contrib.modules` | Current contrib module build-list dependencies from `go list -m all`. |
| `minimal-core-summary.tsv` | Package footprint for `httpx`, `binding`, `middleware/maxbody`, and `middleware/timeout`. |
| `minimal-core-packages.txt` | Non-stdlib packages reached by the minimal-core package set. |
| `*.added.modules` and `*.removed.modules` | Module diff files when `API_BASE_REF` is set. |

The tag-driven GitHub release workflow separately publishes
`dependency-licenses-root.tsv` and `dependency-licenses-contrib.tsv`. They are
draft-release assets, not local `make dependency-report` files.

For release or pull-request comparison, set an explicit base ref:

```sh
API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make dependency-report
```

The report does not replace vulnerability scanning. Vulnerability status is
owned by `make vuln`, `release-check-summary.json` `vulnerability_evidence`,
`docs/dependency-risk.md`, and `docs/vulnerability-dispositions.tsv`.

The license reports are generated only in the tag-driven release workflow after
the root and contrib SPDX SBOMs exist. Each row records the Go module, selected
version, SPDX expression, report status, and source PURL. `detected` means Syft
provided SPDX metadata; `needs_review` preserves `NOASSERTION` or `LicenseRef-`
metadata; `missing_from_sbom` records a build-list module for which the SBOM did
not provide a matching Go PURL. Neither review status is an allow decision.
Apply [license-policy.md](license-policy.md) before publishing.

## Current Footprint Policy

- The root module is allowed to carry JWT/JWK dependencies for the v3 stable JWT
  middleware, but `docs/auth-dependency-split.md` records the v4 target to move
  JWT/JWK-heavy auth packages out of the simple-core module graph. New simple
  HTTP helpers must not add provider, database, Redis, router, OpenTelemetry
  exporter, or generated-app dependencies.
- Contrib owns adapter-heavy dependencies and keeps supported-adapter drift
  visible through release checks.
- The minimal-core path should remain free of third-party runtime packages
  beyond the root module packages required by `httpx`, `binding`,
  `middleware/maxbody`, and `middleware/timeout`.
- If a dependency diff adds a transitive module, the PR or release note must say
  which direct dependency pulled it in and which package owns the risk.
