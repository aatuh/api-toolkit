# Testing Policy

Audience: maintainers writing or reviewing tests for api-toolkit.

This policy keeps tests deterministic enough to run repeatedly in CI and during
local development. No sleep-flaky tests without fake clocks or bounded retry helper.

## Required Patterns

Do not use `time.Sleep` in Go unit, contract, fuzz, race, or package tests just
to wait for time, goroutines, stores, retries, or network side effects. Prefer a
deterministic signal that proves the behavior happened.

Use fake clocks when production code already accepts time through a clock
interface or function. Root idempotency middleware uses `ports.Clock` with
package-local `fixedClock` and `sequenceClock` test fakes so expiration,
retention, and replay behavior can be asserted without wall-clock waiting.

Use injected sleep when production code owns retry backoff. The contrib
HTTP client exposes `httpclient.Options.Sleep`; tests should pass a no-op or
recording function instead of waiting for real backoff durations.

Use deterministic adapter time controls when the fake service supports them.
The Redis cache adapter tests use miniredis deterministic time advancement
instead of waiting for TTL expiry in wall-clock time.

Use channels, contexts, wait groups, explicit callbacks, or deterministic store
state for goroutine and background-worker coordination. If a test needs a
timeout, use `time.After` only as a deadlock guard around a deterministic
operation, not as the primary synchronization mechanism.

Use a bounded retry loop or helper only when the behavior is truly eventually
consistent and no clock, callback, or event hook exists. The retry must have a
deadline or max attempts, return the last observed error or state, and emit
diagnostic output that identifies the dependency or assertion that did not
settle.

Integration shell scripts may use short `sleep` calls only inside bounded
readiness loops. `examples/reference-saas-api/scripts/integration_check.sh`
uses fixed max attempts and diagnostic failure messages for Postgres, Redis,
MinIO, HTTP readiness, and webhook receiver startup.

## Real PostgreSQL Harness

Run `GOWORK=off GOTOOLCHAIN=local make test-postgres` to test the reusable
contrib PostgreSQL harness against the declared PostgreSQL 18 service. Locally
the target starts and removes one loopback-only Docker container. In CI, the
same target uses the `postgres-contract` service container.

The harness creates a database and schema per test, sets the pool search path,
always drops the database during cleanup, supports rollback-only transactions,
migration application, context-cancellation assertions, and targeted connection
interruption. Parallel tests must use the harness rather than a shared schema.

The target never reads `DATABASE_URL`. Set `API_TOOLKIT_TEST_POSTGRES_DSN` only
for the dedicated local/service-container test endpoint and pair it with
`API_TOOLKIT_TEST_POSTGRES=1`; the harness rejects remote hosts, non-test
credentials, application databases, and unexpected connection parameters. Do
not log the test DSN.

## Review Checklist

Before merging a test change:

- Prefer fake clocks, injected sleep, deterministic service time, or explicit
  synchronization over wall-clock waits.
- Keep `time.After` timeouts as deadlock guards with enough context in the
  failure message to debug the blocked condition.
- Require bounded retry loops to define a deadline or max attempts, preserve the
  last failure, and print useful diagnostic output.
- Document any remaining real sleep in the test or helper with why no
  deterministic hook exists, and keep the duration small.
- Run the focused package test and the smallest relevant repo gate before
  marking the backlog item complete.
