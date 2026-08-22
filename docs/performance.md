# Benchmark Baselines

Audience: maintainers reviewing performance-sensitive changes before release.

Benchmarks are regression baselines, not absolute performance promises. Compare
results on the same machine, Go version, branch base, and benchmark flags.
Release reviewers should keep the raw command output with the pull request or
release evidence when a change intentionally moves allocations, latency, or
generated scaffold size.

## Reproducible Release Baseline

The checked-in release baseline is `v4.0.1` at
`09e0117828c960453e3fb4cd028a02bc3e56ff33`. It records the Go version, OS,
architecture, CPU identity, flags, `ns/op`, `B/op`, and `allocs/op` from the
exact root-tag checkout. Reproduce the root sample from that immutable source:

```sh
git checkout v4.0.1
GOWORK=off GOTOOLCHAIN=local go test ./... \
  -run '^$' -bench='Benchmark' -benchmem -benchtime=1x -count=1
```

`contrib/v4.0.1` is withdrawn: its required root `v4.0.0` dependency checksum
does not verify. Therefore `docs/benchmark-baselines.tsv` deliberately has no
contrib measurements for `v4.0.1`; values from v3 or a replacement module must
not be presented as v4 evidence. The next paired root-and-contrib release must
record numeric measurements for both modules. `make benchmark-baseline-check`
enforces this narrow historical exception and otherwise requires both modules.

## Quick Run

Run the root package baselines:

```sh
GOWORK=off GOTOOLCHAIN=local go test \
  ./binding ./queryparams ./specs ./routecontracts \
  ./middleware/maxbody ./middleware/timeout \
  ./middleware/idempotency ./middleware/ratelimit \
  -run '^$' -bench 'Benchmark' -benchmem
```

Run the contrib baselines only for a new paired release, after its published
module dependency identities verify:

```sh
cd contrib
GOWORK=off GOTOOLCHAIN=local go test \
  ./middleware/openapi ./middleware/requestlog ./cmd/api-toolkit \
  -run '^$' -bench 'Benchmark' -benchmem
```

For a fast smoke check during development, add `-benchtime=1x`. Do not use that
as comparative evidence.

## Benchmark Inventory

| Area | Package | Benchmark |
| --- | --- | --- |
| JSON and query binding | `binding` | `BenchmarkBindingDecodeJSON`, `BenchmarkBindingDecodeQuery` |
| Query shape parsing | `queryparams` | `BenchmarkQueryParamsParseRequestShape` |
| OpenAPI metadata rendering | `specs` | `BenchmarkRegistryOpenAPI100Operations` |
| Route plus operation registration | `routecontracts` | `BenchmarkRouteContractsRegisterAndValidate` |
| Request body size limits | `middleware/maxbody` | `BenchmarkMaxBodyWithinLimit` |
| Cooperative and hard timeouts | `middleware/timeout` | `BenchmarkPropagatorSuccess`, `BenchmarkHardTimeoutSuccess` |
| Idempotency write and replay paths | `middleware/idempotency` | `BenchmarkIdempotencyNew`, `BenchmarkIdempotencyReplay` |
| Rate limiting middleware | `middleware/ratelimit` | `BenchmarkRateLimit` |
| Runtime OpenAPI validation | `contrib/middleware/openapi` | `BenchmarkOpenAPIRequestValidation`, `BenchmarkOpenAPIResponseValidation` |
| Request logging | `contrib/middleware/requestlog` | `BenchmarkRequestLog`, `BenchmarkRequestLogWithHeaders` |
| Generated service scaffold | `contrib/cmd/api-toolkit` | `BenchmarkNewServiceSaaSAPIGeneration` |

## Release-specific Measurements and Thresholds

`docs/benchmark-baselines.tsv` is the machine-readable release baseline. Each
row binds a release tag and commit to its module, package, benchmark name,
measurement environment, flags, observed `ns/op`, `B/op`, `allocs/op`, and
the current `max_bytes_per_op` and `max_allocs_per_op` review thresholds.
The machine-readable time column is `observed_ns_per_op`.

Use the thresholds as a release-review trigger: any benchmark result above
`max_allocs_per_op`, above `max_bytes_per_op`, or more than 20% above the
recorded baseline needs an explicit performance note in the pull request or
release evidence. The note should name the behavior gained, the affected
benchmark row, and whether the threshold should be raised.

## Reference Service Load Baseline

The checked-in reference SaaS API has a separate load-smoke baseline because it
is generated application evidence, not a root or contrib package benchmark. Run
it with:

```sh
GOWORK=off GOTOOLCHAIN=local make reference-service-load
```

The command writes `.ci-result/reference-service-load/status`,
`.ci-result/reference-service-load/summary.json`,
`.ci-result/reference-service-load/summary.md`, and `load-smoke.log`. The
summary records request count, concurrency, throughput, latency percentiles,
heap and total allocation deltas, malloc deltas, per-request allocation
estimates, rate-limit responses, timeouts, unexpected statuses, and the expected
missing-API-key failure behavior for `GET /widgets`.

`docs/reference-service-load-baseline.tsv` is the committed seed baseline for
that local smoke. It records latency, throughput, memory, allocations, expected
failure status, unexpected status count, secret-leak count, and the evidence
command. Use it as release-review context on the same machine and Go toolchain;
do not treat it as a public performance SLA.

## Review Rules

- Benchmark output belongs with the change when it touches middleware hot paths,
  binding/parsing helpers, OpenAPI generation, route-contract registration, or
  generated scaffold contents.
- Treat allocation increases as intentional only when the pull request explains
  the behavior gained and why the cost is acceptable.
- Re-run root and contrib benchmarks after Go toolchain updates before comparing
  numbers across releases. Treat package paths from different module majors as
  distinct identities: moves and splits do not create a direct coverage or
  benchmark delta.
- Keep generated scaffold benchmarks writing to benchmark temp directories only;
  they must not write evidence or generated services into the working tree.
- Re-run `make reference-service-load` after changes to the generated full
  service router, auth/idempotency paths, in-memory app services, or evidence
  scripts before updating `docs/reference-service-load-baseline.tsv`.
