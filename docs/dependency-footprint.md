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

For release or pull-request comparison, set an explicit base ref:

```sh
API_BASE_REF=v3.1.2 GOTOOLCHAIN=local make dependency-report
```

The report does not replace vulnerability scanning. Vulnerability status is
owned by `make vuln`, `release-check-summary.json` `vulnerability_evidence`,
`docs/dependency-risk.md`, and `docs/vulnerability-dispositions.tsv`.

## Current Footprint Policy

- The root module is allowed to carry JWT/JWK dependencies for the v3 stable JWT
  middleware, but new simple HTTP helpers must not add provider, database,
  Redis, router, OpenTelemetry exporter, or generated-app dependencies.
- Contrib owns adapter-heavy dependencies and keeps supported-adapter drift
  visible through release checks.
- The minimal-core path should remain free of third-party runtime packages
  beyond the root module packages required by `httpx`, `binding`,
  `middleware/maxbody`, and `middleware/timeout`.
- If a dependency diff adds a transitive module, the PR or release note must say
  which direct dependency pulled it in and which package owns the risk.
