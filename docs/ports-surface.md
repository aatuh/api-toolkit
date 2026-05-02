# Stable Ports Boundary

This note explains which parts of `github.com/aatuh/api-toolkit/v2/ports` are
meant to stay broadly reusable and which parts remain in v2 only as
compatibility-sensitive surfaces.

## What stays in core

- Generic application boundaries such as logging, clocks, IDs, HTTP, rate limiting, validation, docs, health management, and transaction/query lifecycles stay in core.
- These contracts are the normal template for new `ports` additions: narrow, adapter-neutral, and safe to implement across multiple backends.

## Compatibility-sensitive v2 surfaces

- `ports/billing.go` is stable in v2, but it is currently Stripe-shaped. Hosted checkout, provider customer IDs, billing portal flows, and invoice operations reflect the existing adapter rather than a provider-neutral billing model. The `ports` symbols are now deprecated for new code in favor of `github.com/aatuh/api-toolkit/v2/compat/billing`, which preserves the same v2 contract behind an explicit compatibility import path.
- `ports/database.go` query and transaction interfaces stay in core, but `DatabasePool.Stat` and `DatabaseStats` are stable only as a compatibility surface. They currently mirror pgxpool-style counters, and new code should keep that shape inside compatibility adapters rather than using it directly in generic observability code.
- `response_writer` is another example of a stable but compatibility-sensitive surface: retained for v2 users, not a template for new design.

## Compatibility manifest

The following stable `ports` symbols are compatibility-sensitive. If this list
changes, update this note and `VERSIONING.md` in the same change so the boundary
widening is intentional.

Billing compatibility symbols:

- `ports.BillingPortalFlowAfterCompletion`
- `ports.BillingPortalFlowAfterCompletionType`
- `ports.BillingPortalFlowAfterCompletionTypeRedirect`
- `ports.BillingPortalFlowData`
- `ports.BillingPortalFlowSubscriptionUpdateConfirm`
- `ports.BillingPortalFlowSubscriptionUpdateConfirmItem`
- `ports.BillingPortalFlowType`
- `ports.BillingPortalFlowTypeSubscriptionUpdateConfirm`
- `ports.BillingPortalSession`
- `ports.BillingPortalSessionInput`
- `ports.BillingProvider`
- `ports.CheckoutSession`
- `ports.CheckoutSessionRequest`
- `ports.Customer`
- `ports.CustomerAddress`
- `ports.CustomerInput`
- `ports.Invoice`
- `ports.InvoiceInput`
- `ports.InvoiceItem`
- `ports.InvoiceItemInput`
- `ports.InvoiceItemUpdate`
- `ports.PaymentMethod`
- `ports.PaymentProvider`
- `ports.Price`
- `ports.SetupIntent`
- `ports.SetupIntentInput`
- `ports.WebhookEvent`

Database stats compatibility symbols:

- `ports.DatabasePool.Stat`
- `ports.DatabaseStats`
- `ports.DatabaseStats.AcquireCount`
- `ports.DatabaseStats.AcquireDuration`
- `ports.DatabaseStats.AcquiredConns`
- `ports.DatabaseStats.CanceledAcquireCount`
- `ports.DatabaseStats.ConstructingConns`
- `ports.DatabaseStats.EmptyAcquireCount`
- `ports.DatabaseStats.IdleConns`
- `ports.DatabaseStats.MaxConns`
- `ports.DatabaseStats.NewConnsCount`
- `ports.DatabaseStats.TotalConns`

## Guidance for new code

- If your application needs billing behavior, prefer an app-owned billing port or a dedicated contrib adapter contract unless the existing v2 billing surface is the exact shape you want.
- If you do want the current hosted-checkout and invoicing model in v2, import `github.com/aatuh/api-toolkit/v2/compat/billing` instead of the deprecated billing exports in `ports`.
- If you only need database observability data, depend on `DatabasePoolSnapshotProvider`, adapter `StatSnapshot()` methods, or `SnapshotDatabasePoolStats` instead of the legacy `DatabaseStats` interface.
- Do not add more provider-specific billing fields or more driver-specific database counters to core `ports` in v2 unless compatibility requires it.

## V3 cleanup checklist

The compatibility-sensitive ports cleanup plan is coordinated with
`docs/v3-compatibility-roadmap.md`, which covers idempotency release semantics
and authz constructor validation shims. The consolidated removal matrix lives in
that roadmap and is the executable checklist for current API, preferred v2 API,
v3 action, required tests, and removal conditions.

- [ ] Keep the current `ports/billing.go`, database-stats, and `response_writer`
  exports source-compatible for the rest of v2.
- [ ] If this compatibility manifest changes, update `VERSIONING.md`,
  `docs/v3-compatibility-roadmap.md`, docscheck coverage, and
  `docs/release-notes.md` in the same change.
- [ ] Keep `github.com/aatuh/api-toolkit/v2/compat/billing` as the canonical
  v2 import path for the existing provider-shaped billing model.
- [ ] Prefer narrow v2 additions, such as plain-value database snapshot
  capabilities, instead of widening legacy compatibility surfaces.
- [ ] In v3, remove the deprecated `ports/billing.go` exports after the
  compatibility overlap window, or move the existing hosted-checkout and
  invoicing model into an explicit compatibility package.
- [ ] In v3, remove `DatabasePool.Stat` and `DatabaseStats` from the generic
  pool contract; keep driver-shaped stats wrappers in adapters or compatibility
  packages.
- [ ] In v3, retire `response_writer` from the stable core surface; new JSON,
  Problem Details, capture, or wrapper helpers should live under `httpx` or a
  clearly named compatibility/internal package.
