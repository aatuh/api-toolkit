# Provider Runbook

Provider workflows are app-owned starter code. Local tests use fake providers and checked-in fixtures; live checks are opt-in through `RUN_PROVIDER_LIVE_CHECKS=true make provider-live-check` and must never run from default `make finalize`.

## Replay

- Keep signed webhook fixtures under `testdata/providers`.
- Run `go run ./cmd/provider-replay --fixture-dir testdata/providers` or `make provider-check` before touching live sandbox credentials.
- Reproduce provider callback failures with deterministic fixture payloads before using sandbox credentials.
- Never paste live API keys, webhook secrets, customer IDs, invitation tokens, or callback bodies into issue trackers or logs.

## Failure Modes

- Signature failures: verify the configured secret and provider clock tolerance.
- Tenant mismatch: reject the callback and inspect app-owned billing or identity mappings.
- Provider outage: retry app-owned operations with idempotency keys and keep user-visible errors generic.
