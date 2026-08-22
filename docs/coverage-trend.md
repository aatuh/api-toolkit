# Package Coverage Trend

Audience: maintainers and release reviewers comparing package-level test coverage across published releases.

The machine-readable source is `docs/coverage-trend.tsv`. Coverage is a review signal, not a substitute for behavior, contract, race, fuzz, or security tests.

## Method

Each snapshot is measured at the tagged release commit with `GOWORK=off`, `GOTOOLCHAIN=local`, and the repository coverage command. Historical v3 snapshots were backfilled from the tagged source because early releases only retained aggregate coverage logs. `no statements`, `no test files`, and `not reported` are not numeric values and do not contribute to a percentage delta.

`v4.0.1` is a root-only historical baseline: its root rows were measured at the immutable root tag, but `contrib/v4.0.1` is withdrawn because its required root `v4.0.0` dependency checksum does not verify. Its contrib rows intentionally remain `release-integrity-blocked` and `not-reported`; they are neither zero coverage nor a successful contrib measurement. Do not substitute v3 or replacement-module values. A new paired release must provide numeric root and contrib snapshots. Module-major path changes, package moves, and splits are separate package identities, so cross-major values are not direct deltas.

Record the next release snapshot before tagging after `make coverage-check` succeeds:

```sh
COVERAGE_TREND_RELEASE=vX.Y.Z \
COVERAGE_TREND_COMMIT="$(git rev-parse --verify HEAD)" \
GOTOOLCHAIN=local make coverage-trend-record
GOTOOLCHAIN=local make coverage-trend-check
```

## Aggregate Trend

| Release | Commit | Root | Contrib |
| --- | --- | ---: | ---: |
| v3.0.0 | `990489dbcc2c3388bbc1c7f28a2cd9d2182b3434` | 73.2% | 66.1% |
| v3.1.0 | `279dadf0eac51635e6eb7a02c71ae9285d20f66b` | 73.2% | 66.3% |
| v3.1.2 | `85150460b7427b9b5ea14e01d22f577be90edd43` | 77.5% | 68.8% |
| v4.0.1 | `09e0117828c960453e3fb4cd028a02bc3e56ff33` | 71.6% | n/a |

## Package Trend

The scope follows the package dashboard: stable and compatibility-only root packages plus the selected supported contrib adapters that have explicit floors. Values are ordered by release, and `Delta` compares the earliest and latest numeric values.

| Module | Package | Status | v3.0.0 | v3.1.0 | v3.1.2 | v4.0.1 | Delta |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres` | `supported-adapter` | 90.2% | 90.2% | 90.2% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis` | `supported-adapter` | 74.3% | 74.3% | 74.3% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotencyredis` | `supported-adapter` | 68.2% | 68.2% | 68.2% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3` | `supported-adapter` | 75.1% | 75.1% | 75.1% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres` | `supported-adapter` | 80.0% | 80.0% | 80.0% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres` | `supported-adapter` | 86.4% | 86.4% | 86.4% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool` | `supported-adapter` | 49.1% | 49.1% | 71.7% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimitredis` | `supported-adapter` | 65.4% | 65.4% | 65.4% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres` | `supported-adapter` | 81.5% | 81.5% | 93.5% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/bootstrap` | `supported-adapter` | 71.5% | 71.7% | 71.7% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc` | `supported-adapter` | 76.6% | 76.6% | 76.6% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` | `supported-adapter` | 74.3% | 74.3% | 82.4% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi` | `supported-adapter` | 63.0% | 69.2% | 86.2% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/middleware/oteltrace` | `supported-adapter` | 66.1% | 66.1% | 92.2% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog` | `supported-adapter` | 69.0% | 69.0% | 81.2% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery` | `supported-adapter` | 66.7% | 66.7% | 88.8% | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/auditpostgres` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/cacheredis` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotencyredis` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/objectstores3` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/operationpostgres` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/outboxpostgres` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/ratelimitredis` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/bootstrap` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/oidc` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/middleware/metrics` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/middleware/openapi` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/middleware/oteltrace` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/middleware/requestlog` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| contrib | `github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery` | `supported-adapter` | n/a | n/a | n/a | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/apiclient` | `stable` | 93.2% | 93.2% | 93.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/apitest` | `stable` | 78.2% | 78.2% | 78.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/authorization` | `stable` | 98.4% | 98.4% | 98.4% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/binding` | `stable` | 70.8% | 70.8% | 70.8% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/compat/billing` | `compatibility-only` | no statements | no statements | no statements | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/contracttest` | `stable` | 83.9% | 83.9% | 83.9% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/email` | `stable` | no statements | no statements | no statements | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/endpoints/docs` | `stable` | 59.1% | 59.1% | 70.5% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/endpoints/health` | `stable` | 62.9% | 62.9% | 83.1% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/endpoints/list` | `stable` | 77.2% | 77.2% | 77.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/endpoints/pprof` | `stable` | 93.3% | 93.3% | 93.3% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/endpoints/version` | `stable` | 50.0% | 50.0% | 50.0% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/fielderrors` | `stable` | 100.0% | 100.0% | 100.0% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/httpcache` | `stable` | 78.1% | 78.1% | 78.1% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/httpx` | `stable` | 63.7% | 63.7% | 63.7% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/httpx/identity` | `stable` | 45.7% | 45.7% | 75.3% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/httpx/recover` | `stable` | 53.3% | 53.3% | 55.4% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/idempotent` | `stable` | 77.8% | 77.8% | 77.8% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey` | `stable` | 79.7% | 79.7% | 79.7% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/auth/authz` | `stable` | 77.6% | 77.6% | 77.6% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/auth/jwt` | `stable` | 40.0% | 40.0% | 93.3% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant` | `stable` | 84.0% | 84.0% | 84.0% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/deprecation` | `stable` | 83.3% | 83.3% | 83.3% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/idempotency` | `stable` | 71.6% | 71.6% | 71.6% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/json` | `stable` | 55.9% | 55.9% | 70.6% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/maxbody` | `stable` | 28.6% | 28.6% | 85.7% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/querylimits` | `stable` | 51.7% | 51.7% | 95.0% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/ratelimit` | `stable` | 68.6% | 68.6% | 68.6% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/secure` | `stable` | 76.4% | 76.4% | 76.4% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/timeout` | `stable` | 81.9% | 81.9% | 79.4% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/middleware/trace` | `stable` | 80.2% | 80.2% | 80.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/negotiation` | `stable` | 83.8% | 83.8% | 83.8% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/oauth2` | `stable` | 97.2% | 97.2% | 97.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/operations` | `stable` | 77.1% | 77.1% | 77.1% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/ports` | `stable` | 62.5% | 62.5% | 62.5% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/queryparams` | `stable` | 86.7% | 86.7% | 86.7% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/routecontracts` | `stable` | 83.9% | 83.9% | 83.9% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/routepolicy` | `stable` | 73.7% | 73.7% | 73.7% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/scheduler` | `stable` | 89.9% | 89.9% | 89.9% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/scheduler/migrations` | `compatibility-only` | no statements | no statements | no statements | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/securityprofile` | `stable` | 66.3% | 66.3% | 84.9% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/specs` | `stable` | 82.9% | 82.9% | 82.9% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/swagstub` | `compatibility-only` | 68.2% | 68.2% | 68.2% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/upload` | `stable` | 95.8% | 95.8% | 95.8% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v3/webhooks` | `stable` | 80.0% | 80.0% | 80.0% | n/a | n/a |
| root | `github.com/aatuh/api-toolkit/v4/apiclient` | `stable` | n/a | n/a | n/a | 93.2% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/apitest` | `stable` | n/a | n/a | n/a | 78.2% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/authorization` | `stable` | n/a | n/a | n/a | 98.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/binding` | `stable` | n/a | n/a | n/a | 73.5% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/compat/billing` | `compatibility-only` | n/a | n/a | n/a | no statements | n/a |
| root | `github.com/aatuh/api-toolkit/v4/contracttest` | `stable` | n/a | n/a | n/a | 83.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/email` | `stable` | n/a | n/a | n/a | no statements | n/a |
| root | `github.com/aatuh/api-toolkit/v4/endpoints/docs` | `stable` | n/a | n/a | n/a | 70.7% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/endpoints/health` | `stable` | n/a | n/a | n/a | 83.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/endpoints/list` | `stable` | n/a | n/a | n/a | 77.2% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/endpoints/pprof` | `stable` | n/a | n/a | n/a | 93.3% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/endpoints/version` | `stable` | n/a | n/a | n/a | 66.7% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/fielderrors` | `stable` | n/a | n/a | n/a | 100.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/httpcache` | `stable` | n/a | n/a | n/a | 78.1% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/httpx` | `stable` | n/a | n/a | n/a | 63.7% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/httpx/identity` | `stable` | n/a | n/a | n/a | 76.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/httpx/recover` | `stable` | n/a | n/a | n/a | 55.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/idempotent` | `stable` | n/a | n/a | n/a | 77.8% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/auth/apikey` | `stable` | n/a | n/a | n/a | 80.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/auth/authz` | `stable` | n/a | n/a | n/a | 77.6% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/auth/tenant` | `stable` | n/a | n/a | n/a | 84.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/deprecation` | `stable` | n/a | n/a | n/a | 83.3% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/idempotency` | `stable` | n/a | n/a | n/a | 71.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/json` | `stable` | n/a | n/a | n/a | 70.6% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/maxbody` | `stable` | n/a | n/a | n/a | 85.7% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/querylimits` | `stable` | n/a | n/a | n/a | 96.7% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/ratelimit` | `stable` | n/a | n/a | n/a | 68.6% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/secure` | `stable` | n/a | n/a | n/a | 76.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/timeout` | `stable` | n/a | n/a | n/a | 79.4% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/middleware/trace` | `stable` | n/a | n/a | n/a | 81.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/negotiation` | `stable` | n/a | n/a | n/a | 85.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/operations` | `stable` | n/a | n/a | n/a | 77.1% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/ports` | `stable` | n/a | n/a | n/a | 0.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/queryparams` | `stable` | n/a | n/a | n/a | 91.6% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/routecontracts` | `stable` | n/a | n/a | n/a | 86.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/routepolicy` | `stable` | n/a | n/a | n/a | 74.0% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/scheduler` | `stable` | n/a | n/a | n/a | 89.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/scheduler/migrations` | `compatibility-only` | n/a | n/a | n/a | no statements | n/a |
| root | `github.com/aatuh/api-toolkit/v4/securityprofile` | `stable` | n/a | n/a | n/a | 84.9% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/specs` | `stable` | n/a | n/a | n/a | 83.5% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/swagstub` | `compatibility-only` | n/a | n/a | n/a | 68.2% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/upload` | `stable` | n/a | n/a | n/a | 93.2% | n/a |
| root | `github.com/aatuh/api-toolkit/v4/webhooks` | `stable` | n/a | n/a | n/a | 81.7% | n/a |
