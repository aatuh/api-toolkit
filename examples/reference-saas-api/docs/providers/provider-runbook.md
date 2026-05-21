# Provider Runbook

Provider workflows are app-owned starter code. Local tests use fake providers
and checked-in fixtures; live checks are opt-in through
`RUN_PROVIDER_LIVE_CHECKS=true make provider-live-check` and must never run
from default `make finalize`.

## Local Verification

- Keep signed webhook fixtures under `testdata/providers`.
- Run `go run ./cmd/provider-replay --fixture-dir testdata/providers` or
  `make provider-check` before touching live sandbox credentials.
- Reproduce provider callback failures with deterministic fixture payloads
  before using sandbox credentials.
- Never paste live API keys, webhook secrets, customer IDs, invitation tokens,
  or callback bodies into issue trackers or logs.

Expected local evidence:

- `make provider-check` passes with fake providers.
- `cmd/provider-replay` prints sanitized fixture summaries only.
- Tenant mismatch and signature-failure fixtures stay rejected.

## Live Sandbox Checks

Live checks are operator-initiated only:

```sh
RUN_PROVIDER_LIVE_CHECKS=true make provider-live-check
```

Before running them, confirm credentials are loaded from a secret manager or
local shell environment, not committed files. Afterward, rotate any temporary
sandbox credential that was shared outside the normal secret path.

## Failure Modes

- Signature failures: verify the configured secret and provider clock tolerance.
- Tenant mismatch: reject the callback and inspect app-owned billing or identity mappings.
- Provider outage: retry app-owned operations with idempotency keys and keep user-visible errors generic.

## Triage

1. Reproduce with `testdata/providers` fixtures and `cmd/provider-replay`.
2. Check whether the failure is signature verification, stale provider metadata,
   tenant mapping, idempotency collision, provider timeout, or downstream
   persistence.
3. Record the fixture name, command, sanitized event type, tenant mapping result,
   and outcome. Do not record raw callback bodies or secrets.
4. Use live sandbox credentials only after fixture reproduction fails to explain
   the issue.

## Evidence To Record

| Evidence | Record |
| --- | --- |
| Local replay | Command, fixture name, sanitized summary, and pass/fail. |
| Provider tests | `make provider-check` result and package list. |
| Live sandbox | Opt-in command, provider sandbox account, sanitized request ID, and result. |
| Incident follow-up | Secret rotation decision, tenant mapping fix, retry/idempotency result, and user-visible message review. |
