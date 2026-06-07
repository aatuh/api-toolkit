# Benchmark Baselines

Audience: maintainers reviewing performance-sensitive changes before release.

Benchmarks are regression baselines, not absolute performance promises. Compare
results on the same machine, Go version, branch base, and benchmark flags.
Release reviewers should keep the raw command output with the pull request or
release evidence when a change intentionally moves allocations, latency, or
generated scaffold size.

## Quick Run

Run the root package baselines:

```sh
GOWORK=off GOTOOLCHAIN=local go test \
  ./binding ./queryparams ./specs ./routecontracts \
  ./middleware/maxbody ./middleware/timeout \
  ./middleware/idempotency ./middleware/ratelimit \
  -run '^$' -bench 'Benchmark' -benchmem
```

Run the contrib baselines:

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

## Allocation Thresholds

`docs/benchmark-baselines.tsv` is the machine-readable allocation baseline for
the benchmarks above. Each row records the module, package, benchmark name,
observed `B/op`, observed `allocs/op`, and the current `max_bytes_per_op` and
`max_allocs_per_op` review thresholds.

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
  numbers across releases.
- Keep generated scaffold benchmarks writing to benchmark temp directories only;
  they must not write evidence or generated services into the working tree.
- Re-run `make reference-service-load` after changes to the generated full
  service router, auth/idempotency paths, in-memory app services, or evidence
  scripts before updating `docs/reference-service-load-baseline.tsv`.
