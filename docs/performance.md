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
