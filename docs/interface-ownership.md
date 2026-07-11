# Interface Ownership

Audience: maintainers deciding whether an exported interface belongs in root
stable API, contrib, tests, or app-owned code.

Default rule: the package that consumes an interface should own it. Root
interfaces remain only when they are already part of the v3 compatibility
surface, are implemented by users, or are needed by multiple stable packages.

## Ownership Table

| Interface | Owner classification | Notes |
| --- | --- | --- |
| `authorization.Owner` | implemented by user | Domain resource ownership shape. |
| `authorization.TenantOwned` | implemented by user | Domain tenant ownership shape. |
| `compat/billing.PaymentProvider` | compatibility-only adapter-owned | Hosted-checkout compatibility surface; new provider workflows should be app-owned or contrib-owned. |
| `compat/billing.BillingProvider` | compatibility-only adapter-owned | Broad v3 billing compatibility surface, not a new generic boundary. |
| `email.Sender` | implemented by adapter or app | Tiny email send boundary; provider details stay contrib or app-owned. |
| `endpoints/health.DurationLoader` | implemented by user or config adapter | Reads health cache duration from app config. |
| `endpoints/health.RefreshManager` | implemented by scheduler/integration | Refresh hook for health status. |
| `endpoints/health.Checker` | package-local v3 alias | New health integrations should use this alias instead of `ports.HealthChecker`. |
| `endpoints/health.ManagerContract` | package-local v3 alias | New health integrations should use this alias instead of `ports.HealthManager`. |
| `endpoints/health.DetailedManager` | package-local v3 alias | Optional detailed health capability alias. |
| `endpoints/health.CachedManager` | package-local v3 alias | Optional cached health capability alias. |
| `endpoints/health.RouteRegistrar` | package-local v3 alias | Minimal health endpoint registration alias. |
| `endpoints/list.CursorCodec` | implemented by package or user | Cursor encoding boundary for list endpoints. |
| `endpoints/pprof.Router` | implemented by router adapter | Minimal route registration shape. |
| `endpoints/docs.Provider` | package-local v3 alias | New docs integrations should use this alias instead of `ports.DocsProvider`. |
| `endpoints/docs.ManagerContract` | package-local v3 alias | New docs integrations should use this alias instead of `ports.DocsManager`. |
| `endpoints/docs.HTMLModeProvider` | package-local v3 alias | Optional docs HTML mode capability alias. |
| `endpoints/docs.RouteRegistrar` | package-local v3 alias | Minimal docs endpoint registration alias. |
| `fielderrors.Provider` | implemented by validation errors | Exposes field errors for Problem Details mapping. |
| `middleware/auth/apikey.Verifier` | implemented by user or auth adapter | API key verification belongs to app/provider. |
| `middleware/idempotency.LegacyInFlightCompatibilityEventSink` | implemented by operators | Compatibility telemetry sink. |
| `middleware/idempotency.LegacyInFlightCompatibilityMetricSink` | implemented by operators | Compatibility metric sink. |
| `oauth2.Validator` | implemented by user or auth adapter | Token validation boundary; provider-specific code stays out of root. |
| `ports.Authorizer` | compatibility-sensitive app-owned | Prefer package-local or app-owned authorization interfaces for new code. |
| `ports.CachedHealthManager` | compatibility-sensitive adapter-owned | Root health compatibility surface. |
| `ports.Clock` | implemented by adapter or tests | Time source for deterministic behavior. |
| `ports.CORSHandler` | compatibility-sensitive adapter-owned | Prefer contrib CORS adapter or app-owned middleware. |
| `ports.DatabaseConnection` | compatibility-sensitive adapter-owned | Database shape retained for v3 compatibility. |
| `ports.DatabasePool` | compatibility-sensitive adapter-owned | Database pool shape retained for v3 compatibility. |
| `ports.DatabasePoolSnapshotProvider` | compatibility-sensitive adapter-owned | Optional stats snapshot provider. |
| `ports.DatabaseResult` | compatibility-sensitive adapter-owned | Database result shape. |
| `ports.DatabaseRow` | compatibility-sensitive adapter-owned | Database row shape. |
| `ports.DatabaseRows` | compatibility-sensitive adapter-owned | Database rows shape with lifecycle ownership. |
| `ports.DatabaseTransaction` | compatibility-sensitive adapter-owned | Database transaction shape. |
| `ports.DetailedHealthManager` | compatibility-sensitive adapter-owned | Detailed health should stay operator-only. |
| `ports.DocsHTMLModeProvider` | compatibility-sensitive adapter-owned | Docs HTML mode provider. |
| `ports.DocsManager` | compatibility-sensitive adapter-owned | Docs manager compatibility surface. |
| `ports.DocsProvider` | compatibility-sensitive adapter-owned | Docs provider compatibility surface. |
| `ports.EnvVar` | compatibility-sensitive adapter-owned | Env lookup boundary. |
| `ports.HealthChecker` | implemented by user or adapter | Readiness/liveness check boundary. |
| `ports.HealthCheckRegistry` | compatibility-sensitive adapter-owned | Health registry compatibility surface. |
| `ports.HealthManager` | compatibility-sensitive adapter-owned | Health manager compatibility surface. |
| `ports.HTTPClient` | implemented by user or adapter | Outbound HTTP client boundary. |
| `ports.HTTPMiddleware` | compatibility-sensitive router-owned | Prefer concrete middleware functions for new code. |
| `ports.HTTPRouter` | compatibility-sensitive router-owned | Router adapter boundary. |
| `ports.IDGen` | implemented by adapter or tests | ID generation boundary. |
| `ports.IdempotencyReservationReleaser` | implemented by adapter | Token-aware idempotency reservation cleanup. |
| `ports.IdempotencyStore` | implemented by adapter | Idempotency store boundary. |
| `ports.Logger` | implemented by adapter or app | Logging boundary; keep values bounded and redacted. |
| `ports.MethodRouteRegistrar` | compatibility-sensitive router-owned | Minimal method route registration. |
| `ports.Middleware` | compatibility-sensitive middleware-owned | Prefer concrete `Handler` methods in new code. |
| `ports.MiddlewareChain` | compatibility-sensitive router-owned | Router chain compatibility surface. |
| `ports.Migrator` | compatibility-sensitive adapter-owned | Migration lifecycle boundary. |
| `ports.PolicyEngine` | implemented by adapter or app | Policy engine boundary. |
| `ports.RateLimiter` | implemented by adapter | Rate limiter boundary. |
| `ports.ReservationReleasableIdempotencyStore` | implemented by adapter | Composed idempotency store and releaser. |
| `ports.TxManager` | compatibility-sensitive adapter-owned | Transaction manager boundary. |
| `ports.URLParamExtractor` | implemented by router adapter | URL parameter extraction boundary. |
| `ports.Validator` | implemented by adapter | Validation boundary. |
| `routecontracts.Policy` | implemented by package or user | Route registration policy hook. |
| `routecontracts.Router` | implemented by router adapter | Route contract registry boundary. |
| `scheduler.LastRunProvider` | implemented by adapter | Scheduler persistence read boundary. |
| `scheduler.Logger` | implemented by app or adapter | Scheduler logging boundary. |
| `scheduler.Recorder` | implemented by adapter | Scheduler persistence write boundary. |
| `scheduler.RecorderFailureHandler` | implemented by app | Failure hook for recorder persistence. |
| `webhooks.Signer` | implemented by package or user | Outbound webhook signing boundary. |
| `webhooks.Verifier` | implemented by package or user | Incoming webhook verification boundary. |

## Review Rules

- Challenge any new exported interface with fewer than two real implementations.
- Prefer app-owned interfaces for business-specific behavior.
- Prefer contrib-owned interfaces for provider, database, router, telemetry, or
  generated-service behavior.
- Use `docs/api-review-checklist.md` before adding root interfaces.
- Keep `docs/api-inventory.md` current when interface symbols change.
