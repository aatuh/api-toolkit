# Global State Audit

Audience: maintainers reviewing stable or compatibility-only package-level
state.

This audit covers package-level `var` declarations in root packages classified
as `stable` or `compatibility-only`. Constructors and request handlers must not
depend on undocumented mutable global state. Exported preset vars are retained
for compatibility and should be treated as immutable defaults; copy values into
local configuration before customization.

| Package | Symbol | Classification | Mutation Policy | Concurrency Evidence |
| --- | --- | --- | --- | --- |
| `github.com/aatuh/api-toolkit/v3/endpoints/docs` | `defaultHTMLTemplate` | precompiled-template | Initialized once; do not reassign. | `html/template.Template` is safe for concurrent execution after parse. |
| `github.com/aatuh/api-toolkit/v3/endpoints/docs` | `staticHTMLTemplate` | precompiled-template | Initialized once; do not reassign. | `html/template.Template` is safe for concurrent execution after parse. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `ErrForbidden` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `ErrHeaderBytesExceeded` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `ErrHeaderCountExceeded` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `ErrTooManyRequests` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `ErrUnauthorized` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `HeaderLimitsBalanced` | immutable-default | Exported compatibility preset; treat as read-only and copy before customization. | Plain value has no internal mutable references. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `HeaderLimitsRelaxed` | immutable-default | Exported compatibility preset; treat as read-only and copy before customization. | Plain value has no internal mutable references. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `HeaderLimitsStrict` | immutable-default | Exported compatibility preset; treat as read-only and copy before customization. | Plain value has no internal mutable references. |
| `github.com/aatuh/api-toolkit/v3/httpx` | `defaultTypeRegistry` | private-default-registry | Private registry initialized once; no exported mutator exposes this instance. | Package uses it for URI lookup only after initialization. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey` | `errMalformedCredential` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey` | `errMissingCredential` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey` | `errMultipleCredentials` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/apikey` | `errVerifierMissing` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/authz` | `ErrRequireRoleMissingResolver` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/authz` | `ErrRequireRoleMissingRole` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant` | `errNoSourcesConfigured` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant` | `errTenantHeaderInvalid` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant` | `errTenantMismatch` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/auth/tenant` | `errTenantMissing` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with internal matching. |
| `github.com/aatuh/api-toolkit/v3/middleware/idempotency` | `ErrLegacyInFlightClockSkewPreflightRisk` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/idempotency` | `ErrLegacyInFlightTTLMismatch` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/idempotency` | `errBodyTooLarge` | sentinel-error | Package-private sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/timeout` | `ErrHardTimeoutCaptureLimitExceeded` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/middleware/timeout` | `defaultHardTimeoutCaptureOverflowProblem` | immutable-default | Private Problem Details default; do not reassign. | Plain value is copied on write paths. |
| `github.com/aatuh/api-toolkit/v3/middleware/timeout` | `defaultHardTimeoutPanicProblem` | immutable-default | Private Problem Details default; do not reassign. | Plain value is copied on write paths. |
| `github.com/aatuh/api-toolkit/v3/middleware/timeout` | `defaultHardTimeoutProblem` | immutable-default | Private Problem Details default; do not reassign. | Plain value is copied on write paths. |
| `github.com/aatuh/api-toolkit/v3/middleware/trace` | `cryptoRandRead` | test-hook | Private generator hook; production code treats it as immutable, tests restore it after override. | Not exported; request path reads function pointer only. |
| `github.com/aatuh/api-toolkit/v3/middleware/trace` | `fallbackIDCounter` | atomic-counter | Private fallback counter; do not reset outside tests. | Uses `atomic.Uint64`. |
| `github.com/aatuh/api-toolkit/v3/operations` | `ErrInvalidState` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with validation errors. |
| `github.com/aatuh/api-toolkit/v3/operations` | `ErrInvalidTransition` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with validation errors. |
| `github.com/aatuh/api-toolkit/v3/ports` | `ErrLegacyInFlightReservationMissingToken` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/ports` | `ErrLegacyInFlightTokenMismatch` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/ports` | `ErrResourceMissing` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/routepolicy` | `ErrUnsupportedPolicy` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/scheduler/migrations` | `Migrations` | embedded-fs | Generated embedded filesystem; do not reassign. | `embed.FS` is immutable after initialization. |
| `github.com/aatuh/api-toolkit/v3/specs` | `AllEndpoints` | immutable-default | Exported compatibility endpoint group; treat as read-only. | Plain endpoint strings copied into struct fields. |
| `github.com/aatuh/api-toolkit/v3/specs` | `DocsEndpoints` | immutable-default | Exported compatibility endpoint group; treat as read-only. | Plain endpoint strings copied into struct fields. |
| `github.com/aatuh/api-toolkit/v3/specs` | `HealthEndpoints` | immutable-default | Exported compatibility endpoint group; treat as read-only. | Plain endpoint strings copied into struct fields. |
| `github.com/aatuh/api-toolkit/v3/specs` | `PprofEndpoints` | immutable-default | Exported compatibility endpoint group; treat as read-only. | Plain endpoint strings copied into struct fields. |
| `github.com/aatuh/api-toolkit/v3/specs` | `SystemEndpoints` | immutable-default | Exported compatibility endpoint group; treat as read-only. | Plain endpoint strings copied into struct fields. |
| `github.com/aatuh/api-toolkit/v3/specs` | `componentNamePattern` | precompiled-regexp | Private regexp initialized once; do not reassign. | `regexp.Regexp` is safe for concurrent matching. |
| `github.com/aatuh/api-toolkit/v3/swagstub` | `registry` | synchronized-registry | Compatibility registry is mutable only through `Register`; keep direct map access private. | Protected by `registryMu` for reads and writes. |
| `github.com/aatuh/api-toolkit/v3/swagstub` | `registryMu` | synchronized-registry | Mutex protecting the compatibility registry; do not reassign. | `sync.RWMutex` guards `registry`. |
| `github.com/aatuh/api-toolkit/v3/webhooks` | `ErrInvalidSignature` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
| `github.com/aatuh/api-toolkit/v3/webhooks` | `ErrMissingSignature` | sentinel-error | Stable error sentinel; do not reassign. | Immutable error value used with `errors.Is`. |
