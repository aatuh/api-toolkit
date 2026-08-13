# Provider live evidence

The normal test suite is hermetic: it uses signed fixtures, fake providers, and
local JWKS servers. It never calls a provider. This is intentional—provider
payloads and credentials remain untrusted, and ordinary pull requests cannot
access protected sandbox secrets.

The scheduled `nightly / provider-sandbox` job is the separate, protected
evidence path. It runs only in the GitHub `provider-sandbox` environment and
writes the sanitized artifact
`.ci-result/provider-live/evidence.json`. The artifact contains only the check
time, provider name, and one of these states:

| State | Meaning |
| --- | --- |
| `passed` | A configured, non-mutating sandbox reachability check completed. |
| `failed` | A configured check did not complete; inspect the protected job, not the artifact, for details. |
| `skipped_no_credentials` | That provider was not configured. This is not success. |
| `not_requested` | An operator did not opt in locally. This is not success. |

The checked paths are Stripe price listing, Resend domain reachability, and
Clerk JWKS retrieval. They deliberately create no customer, payment, email, or
identity resource, so no provider-side cleanup is needed. App-owned tests that
create sandbox resources must use a unique run prefix, clean up each created
resource, and publish only this same sanitized result shape.

Configure `STRIPE_SANDBOX_SECRET_KEY`, `RESEND_SANDBOX_API_KEY`, and
`CLERK_SANDBOX_JWKS_URL` as secrets in the protected environment. Do not add
them to repository, workflow output, artifacts, logs, issue comments, or local
`.env` files. If a secret is absent, the job uploads `skipped_no_credentials`
and does not claim provider success.

Run the same check locally only when intentionally using a sandbox account:

```sh
RUN_PROVIDER_LIVE_CHECKS=true GOWORK=off GOTOOLCHAIN=local make provider-live-check
```

The evidence artifact is retained for 90 days. The supported-adapter policy
requires a successful result no more than 30 days old before a provider adapter
may be described as having current sandbox evidence. A missing, skipped, stale,
or failed result does not invalidate hermetic fixture coverage, but it must be
called out as `hermetic-provider-fixture+manual-real-service` rather than
current live-provider evidence.
