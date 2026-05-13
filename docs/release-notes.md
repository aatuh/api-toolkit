# Release Notes

Audience: release consumers and maintainers who need dated behavior changes,
upgrade notes, and package-tied compatibility acknowledgements.

## Release checklist

For stable surface changes, deprecations, or compatibility-sensitive updates,
keep this file focused on user-visible behavior and upgrade notes. The command
source of truth is `docs/release-runbook.md`.

- Update `VERSIONING.md`, public docs, and package docs that describe the affected stability contract.
- Update `scripts/apicheck.sh` and docscheck coverage when the stable package list or compatibility-sensitive manifest changes.
- Update `docs/ports-surface.md`, `docs/v3-compatibility-roadmap.md`, release notes, and upgrade notes when compatibility-sensitive ports or legacy stable surfaces change.
- Add release notes and upgrade notes that describe user-visible behavior, migration paths, and compatibility impact.
- Run release evidence through the runbook path; `API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-check` is the release-readiness gate, while `make finalize` and `make audit-check` are local/reviewer gates.
- Run `API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make contrib-api-drift-report` when selected contrib adapters or integrations change exported APIs; selected packages come from `docs/contrib-api-drift-packages.txt`, supported-adapter incompatible drift is gate-enforced, and this does not make contrib stable.
- Run `CONTRIB_RELEASE_BASE_REF=v2.1.0 GOTOOLCHAIN=local make contrib-release-notes-check` when supported contrib adapter, integration, middleware, bootstrap, or telemetry behavior files or runtime assets change.
- Supported-adapter contrib packages remain outside the stable core API promise, but incompatible public API drift in that tier must be treated as gate-enforced and resolved with compatibility, reclassification, or a major-release policy decision.
- If there is incompatible report-only contrib drift, add an explicit release note or upgrade note acknowledgement tied to the affected package. This does not make contrib stable.
- Update `docs/vulnerability-dispositions.tsv` when imported-only vulnerability IDs change, expire, or receive upgraded dependencies.
- Update `docs/contrib-api-drift-dispositions.tsv` when current contrib drift packages or incompatible drift status changes.
- Use clean publication evidence with `API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence`; reserve `ALLOW_DIRTY_RELEASE_EVIDENCE=1 API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make release-evidence` for local dirty-tree audit evidence that is not acceptable before publishing.
- Use `docs/release-manifests.md` when interpreting `docs/package-classification.tsv`, `docs/contrib-api-drift-dispositions.tsv`, and `docs/vulnerability-dispositions.tsv`.

## 2026-05-02

### Correctness, security, and release governance

- `specs.Operation` now includes `OperationID` and emits OpenAPI
  `operationId` values so route contracts can carry stable client-visible
  operation identity.
- `routepolicy` now includes typed metadata helpers for auth, deprecation,
  sunset, tenant, idempotency, rate-limit, admin-policy, and Problem Details
  response contracts, plus operation linting for missing production policy
  metadata.
- `routepolicy` now exposes typed metadata readers for auth, deprecation,
  tenant, idempotency, rate-limit, admin-policy, and Problem Details response
  contracts. Contract linting now requires unsafe-write tenant and idempotency
  metadata to be explicitly marked `required: true` instead of accepting any
  extension value.
- `routepolicy.LintOperations` and `api-toolkit contracts lint` now fail
  non-public operations without security metadata and unsafe write operations
  without tenant, idempotency, rate-limit, and Problem Details policy metadata,
  while allowing known public readiness, liveness, docs, and version routes.
- `api-toolkit contracts lint` now accepts repeatable `--public-path` and
  `--admin-path` flags so applications can extend the default public and
  operator-only path sets without weakening the built-in production checks.
- `routepolicy`, `contracttest`, and `api-toolkit contracts lint` now enforce
  unique OpenAPI `operationId` values so generated clients and compatibility
  reviews can rely on stable operation identity.
- `routepolicy.LintOperations` and `api-toolkit contracts lint` now require
  non-public operations, including safe reads, to document Problem Details
  error responses.
- `routepolicy.LintOperations` and `api-toolkit contracts lint` now fail
  unsafe write operations that omit request body metadata for POST/PUT/PATCH or
  omit a documented 2xx success response.
- `contracttest` now includes assertions for operation IDs, Problem Details
  error responses, tenant/idempotency/rate-limit/admin policy metadata,
  registry-wide operation ID coverage, and conservative OpenAPI compatibility
  findings.
- `contracttest` now includes stricter generated-OpenAPI assertions for
  expected security scopes, tenant policy source, idempotency header, named
  admin policy, and sets of Problem Details response statuses.
- `contracttest.OpenAPICompatibilityFindings` now reports tenant,
  idempotency, rate-limit, admin-policy, and deprecation/sunset route policy
  drift, matching the stricter `api-toolkit contracts diff` behavior.
- CI now runs `make docs-check` explicitly and runs
  `make contrib-release-notes-check` on pull requests against the fetched PR
  base ref, keeping documentation and supported-contrib release-note governance
  visible before merge.
- CI pull-request governance now also runs `make contrib-api-drift-report`
  against the fetched PR base ref, so supported-adapter incompatible drift
  fails before merge without making contrib part of the stable core API promise.
- `github.com/aatuh/api-toolkit/contrib/v2/bootstrap` now exposes
  `APIService` and `APIServiceConfig` as a supported composition root for
  generated services, with safe admin-wrapper system endpoint mounting and
  startup checks.
- `github.com/aatuh/api-toolkit/contrib/v2/bootstrap.APIServiceConfig` now
  accepts named shutdown hooks so composed services can close auth, telemetry,
  or adapter background resources after the HTTP server stops.
- `middleware/auth/tenant.Options.RequireAllSources` now lets services require
  every configured tenant source to be present and equal before a handler runs,
  which supports authenticated-tenant-to-header mismatch checks.
- `github.com/aatuh/api-toolkit/contrib/v2/cmd/api-toolkit` adds the
  developer CLI with `new service`, `contracts lint`, `contracts diff`, and
  `version` commands. The generated `saas-api` service uses chi-backed
  bootstrap defaults, code-first route contracts, OpenAPI output, public
  readiness, admin-protected metrics/pprof/detailed health, core API-key and
  tenant middleware, and idempotent write behavior, plus a checked-in OpenAPI
  golden workflow.
- Generated `saas-api` services now fail startup under `ENV=production` unless
  `API_KEY` and `ADMIN_KEY` are explicitly set, so local fallback credentials
  cannot be deployed accidentally.
- Generated `saas-api` services now include a `.dockerignore` and a hardened
  multi-stage Dockerfile that runs tests during build, compiles a static binary,
  and runs it from a non-root distroless runtime image instead of `go run` in a
  full Go toolchain image.
- Generated `saas-api` services now include a `.gitignore` that excludes local
  `.env` files, coverage output, temporary directories, test binaries, and the
  built service binary while keeping `.env.example` tracked.
- Generated `saas-api` Makefiles now include `contracts-lint` and
  `contracts-diff` targets backed by the api-toolkit CLI, and generated
  `finalize` runs those contract checks alongside tests and OpenAPI golden
  verification.
- Generated `saas-api` services now keep memory idempotency storage as the local
  default but reject it under `ENV=production`; production defaults to the Redis
  idempotency adapter and requires `REDIS_ADDR` before startup.
- `api-toolkit new service` now supports `--auth jwt` and `--auth clerk` for the
  `saas-api` profile. Generated bearer-token services validate tokens through
  JWKS, require issuer and audience configuration, extract tenant scope from
  validated token claims, enforce route scopes, close auth middleware through
  bootstrap shutdown hooks, and keep generated contract tests and OpenAPI
  goldens aligned. Development-header and unknown modes still fail closed.
- `api-toolkit new service` now supports the explicit `dev-api` profile with
  `--auth dev-headers`. The generated development service requires explicit
  dangerous-bypass environment settings, separates debug user, tenant, and scope
  headers, keeps tenant mismatch and idempotent write tests, and refuses to
  start with dev-header auth when `ENV=production`.
- Generated services now wire `bootstrap.NewDefaultRouterWithConfig` to the
  contrib Prometheus recorder, so protected `/metrics` exposes bounded HTTP
  request counters and histograms instead of only runtime collector output.
- `contrib/middleware/auth/clerk.Subject` now exposes tenant and scope strings
  derived from validated JWT claims while preserving subject comparability, so
  applications and generated services can enforce tenant and route-scope policy
  from Clerk tokens.
- `middleware/idempotency.Options.OnOutcome` now emits bounded request-path
  idempotency outcome events, and `OutcomeEvent.MetricLabels()` exposes only
  method, store class, outcome, and status class for metrics.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics` now records
  bounded `idempotency_outcomes_total` Prometheus counters through
  `IdempotencyOutcomeHook`, and
  `github.com/aatuh/api-toolkit/contrib/v2/middleware/requestlog` now provides
  `IdempotencyOutcomeLogHook` with the same low-cardinality outcome shape.
  Generated `saas-api` services wire both hooks by default.
- `middleware/idempotency` now supports `Options.StorageKeyFunc` plus
  `TenantScopedStorageKeyFunc()` so multi-tenant services can hash
  client-supplied idempotency keys with tenant and actor scope before shared
  storage access. Generated `saas-api` services opt into the helper while
  preserving the original `Idempotency-Key` response header on replay.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics` now records
  bounded `health_status_changes_total` Prometheus counters through
  `HealthStatusChangeHook`, using only `from` and `to` health-status labels for
  scheduler transitions.
- Routes registered through `routecontracts` now attach bounded
  `routepolicy` observability labels. Contrib metrics records them through
  `http_route_policy_requests_total`, and contrib request logging emits
  `policy_*` fields without raw scopes, tenant sources, rate-limit policy
  names, or admin policy names.
- `github.com/aatuh/api-toolkit/contrib/v2/bootstrap.APIServiceConfig` now
  supports named `BackgroundTasks` that run with the service context, fail the
  service on unexpected task errors, and stop during graceful shutdown.
  Generated `saas-api` services use this to run health refreshes with bounded
  health-status metrics.
- `make release-check` now runs `contrib-api-drift-report` as a first-class
  release-readiness subcheck, and release evidence reuses that log for the
  structured contrib drift summary instead of leaving supported-adapter API
  drift only in the evidence-only path.
- `api-toolkit contracts diff` now performs compatibility review over parsed
  OpenAPI operations. Additive operations pass, while removed operations,
  changed operation IDs, removed documented parameters, added required
  parameters, removed documented responses, request-body tightening or content
  removal, response content removal, and changed security requirements fail with
  deterministic findings.
- `api-toolkit contracts diff` now also fails closed when existing operations
  drift in tenant, idempotency, rate-limit, admin-policy, or deprecation/sunset
  route policy metadata.
- `api-toolkit contracts diff` and
  `contracttest.OpenAPICompatibilityFindings` now also flag removed or changed
  `components.securitySchemes`, so auth header, bearer, OAuth, or OIDC
  contract drift is caught even when operation-level security requirements keep
  the same scheme name.
- `api-toolkit contracts diff` now also reviews OpenAPI component schemas and
  reports removed schemas, added required properties, removed object
  properties, type/ref changes, and enum value removals as compatibility
  findings.
- `api-toolkit contracts diff` now applies those conservative schema
  compatibility checks to inline request and response media schemas on existing
  operations, so handler-local contract narrowing is caught before release.
- `api-toolkit contracts lint` now fails when an operation references a
  security requirement that is not defined in `components.securitySchemes`,
  preventing reviewed specs from declaring unenforceable auth.
- `contracttest` now exposes `SecuritySchemeDefinitionFindings` and
  `AssertSecuritySchemesDefined` so service tests can catch the same undefined
  OpenAPI security-scheme references as CLI contract linting.
- `contracttest.OpenAPICompatibilityFindings` now reports the same conservative
  OpenAPI component and inline request/response schema drift findings, so
  library tests and CLI release review stay aligned.
- `github.com/aatuh/api-toolkit/contrib/v2/bootstrap` now exposes middleware
  stage identifiers, strict/dev middleware order helpers, and startup
  validation for custom APIService middleware order declarations.
- `github.com/aatuh/api-toolkit/contrib/v2/bootstrap` now exposes
  `StrictSaaSAPIMiddlewareOrder` for services that require the full production
  policy sequence of auth, tenant, and idempotency after the transport
  middleware stack, and generated `saas-api` services declare that order during
  startup validation.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics` now
  canonicalizes Prometheus HTTP metric labels so methods stay within standard
  HTTP verbs plus `OTHER`/`UNKNOWN`, invalid statuses collapse to `0`, and route
  labels are trimmed before series creation.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/openapi` response
  validation now accepts `ResponseValidationOptions.ShouldValidate` so services
  can skip response buffering for streaming, upgrade, or large-download routes
  while keeping request validation enabled.
- `middleware/timeout.NewHard` now accepts `Options.EventHooks`, emitting
  bounded operator metadata for timeout, panic, and response-capture overflow
  outcomes without exposing panic values, paths, query strings, headers, or
  bodies.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics` now exposes
  `HardTimeoutEventHook` and records bounded hard-timeout outcomes in
  `http_hard_timeout_events_total`; request logging now exposes
  `HardTimeoutEventLogHook` with the same bounded event shape.
- `securityprofile.StreamingRouteOverride` now provides an explicit route-level
  opt-out for streaming, SSE, websocket, or large-download routes that must
  avoid hard-timeout response buffering and preserve optional writer
  interfaces.

- `contrib/adapters/idempotencyredis.ReleaseReservation` now performs atomic
  token-aware compare-and-delete cleanup so stale releasers cannot delete newer
  in-flight reservations after expiry or replacement.
- `middleware/timeout.NewHard` now contains handler panics inside the
  hard-timeout goroutine. Panics before timeout return deterministic Problem
  Details responses, while panics after timeout are contained after the 504
  response has already won.
- `securityprofile.WithHardTimeoutMaxCaptureBytes` and
  `RouteOverride.HardTimeoutMaxCaptureBytes` expose hard-timeout response
  capture limits through global and per-route profile configuration.
- Memory and Redis idempotency adapter legacy recovery events now hash keys by
  default and expose raw keys only through explicit raw-key opt-in fields for
  short incident-review windows.
- The contrib release-note review gate now scopes behavior-change release-note
  requirements to packages classified as `supported-adapter`, preserving
  supported-adapter governance without over-requiring notes for experimental or
  wrapper-only contrib internals.
- The contrib release-note review gate now includes package-owned runtime
  assets such as JSON, YAML, SQL, template, and policy files under supported
  contrib package directories.
- `docs/supported-adapter-contracts.tsv` now defines behavior contracts and
  direct-test/release-drift evidence for every `supported-adapter` contrib
  package. The chi router adapter and zap logger adapter are promoted to
  `supported-adapter` and included in the contrib drift gate.
- `github.com/aatuh/api-toolkit/contrib/v2/adapters/ratelimittest` adds reusable
  rate limiter adapter contract coverage, and `ratelimitredis` now runs it to
  prove empty-key bypass, per-key isolation, retry-after, and refill behavior.
- `github.com/aatuh/api-toolkit/contrib/v2/adapters/healthchecktest` adds
  reusable health checker adapter contract coverage for supported Stripe,
  Resend, and Clerk readiness checks.

### Stable core API additions

- Added stable core packages `binding` and `middleware/auth/apikey` for typed
  request binding, Problem Details-compatible validation errors, API key
  authentication, optional auth, context principals, and scope enforcement.
- `endpoints/list` now includes signed HMAC cursor pagination helpers alongside
  the existing limit/offset APIs.
- `specs.Operation` now supports route contract metadata for parameters,
  security requirements, scopes, deprecation, sunset metadata, request bodies,
  responses, and deterministic OpenAPI extensions.
- `contrib/examples/api-key` demonstrates local-only HMAC-backed API key
  verification and scoped routes.
- Added stable core package `httpcache` for ETag and Last-Modified conditional
  request helpers, including `304 Not Modified` and `412 Precondition Failed`
  response paths.
- Added stable core package `middleware/deprecation` for runtime `Deprecation`,
  `Sunset`, and deprecation-policy `Link` headers.
- Added stable core package `webhooks` for raw-body-preserving HMAC webhook
  verification, JSON event decoding, accepted-event handling, and Problem
  Details failures.
- `specs` now supports reusable OpenAPI schemas, responses, security schemes,
  and schema refs for request and response content.
- Added stable core package `routecontracts` for registering handlers and
  matching OpenAPI operations together.
- Added stable core package `negotiation` for `Accept` and `Content-Type`
  negotiation, including `406` and `415` Problem Details responses.
- `specs` now generates deterministic OpenAPI schemas from Go structs for
  route contract components.
- `httpx` now includes a typed Problem Details catalog for stable
  machine-readable error codes and catalog-backed error mapping.
- Added stable core package `queryparams` for collection sorting, filtering,
  sparse fieldsets, and include parameter parsing without storage coupling.
- Added stable core package `operations` for `202 Accepted` responses and
  pollable asynchronous operation resources.
- `webhooks` now includes outbound HMAC-SHA256 signing helpers for JSON event
  requests that remain compatible with the existing receiver verifier.
- Added stable core package `contracttest` for route contract, OpenAPI,
  generated contract, and problem catalog assertion helpers.
- Added stable core package `routepolicy` and opt-in `routecontracts` policy
  hooks for deriving deprecation headers, content negotiation, auth,
  idempotency, and rate-limit middleware from route operation metadata.
- `specs` can now register reusable Problem Details and validation problem
  components from an `httpx.ProblemCatalog` while preserving unchanged OpenAPI
  output until the catalog helper is used.
- `middleware/ratelimit` can now emit standard `RateLimit-Limit`,
  `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` headers when
  header emission is explicitly enabled.
- Added stable core package `idempotent` for idempotency-key requirements,
  deterministic request hashes, conflict/replay Problem Details, accepted
  replay responses, and OpenAPI operation extensions.
- `webhooks` now includes replay-window checks, required event-id contracts,
  timestamp/event-id header constants, and delivery attempt/result contract
  types without adding retry persistence or provider-specific schemas.
- Added stable core package `upload` for multipart form decoding, required file
  checks, per-file and aggregate size limits, content-type allowlists, and
  Problem Details-compatible field errors.
- Added stable core package `oauth2` for provider-neutral bearer token claims,
  validators, scope checks, JWKS configuration values, OpenAPI security scheme
  registration, and `authorization.Actor`/scope mapping.
- Added stable core package `apitest` for deterministic HTTP API assertions over
  Problem Details, validation fields, headers, pagination, operation-accepted
  responses, webhook signatures, and OpenAPI golden output.
- Added stable core package `apiclient` for client-side Problem Details
  decoding, cursor iteration, `Retry-After` parsing, precondition headers, API
  key transports, webhook signing transports, and JSON request/response helpers.

### Dependency and release evidence updates

- Contrib dependencies were upgraded to burn down the imported-only
  `govulncheck` findings from v39: `github.com/jackc/pgx/v5` is now on
  `v5.9.0`, and `google.golang.org/grpc` is now on `v1.79.3`.
- `docs/dependency-risk.md` now records the v39 advisory ownership map for
  `GO-2026-4762`, `GO-2026-4771`, and `GO-2026-4772`; the active
  `docs/vulnerability-dispositions.tsv` manifest is header-only while current
  imported-only vulnerability evidence is zero.
- The release evidence parser contract now includes a mixed same-package contrib
  drift fixture where one package has both `Incompatible changes:` and
  `Compatible changes:` and must summarize as incompatible.
- Runtime use of the legacy `response_writer` package was removed from
  `httpx/recover` and maintained contrib HTTP middleware. Those packages now use
  package-local response wrappers while the public `response_writer` package
  remains source-compatible for v2 callers.
- `make release-artifact-verify-fixture` now builds a synthetic local release
  asset bundle and runs the local verifier path. This is only local fixture
  coverage; publication verification still requires downloaded GitHub draft
  release assets, `RELEASE_ARTIFACT_VERIFY_MODE=publication`, `RELEASE_TAG`,
  `GITHUB_REPOSITORY`, real Sigstore material, and online attestation checks.
- The release workflow now prints `make release-review-summary` output after
  clean evidence is generated and before release artifact verification steps.

### Contrib behavior and compatibility notes

- `github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders` now
  requires explicit dangerous-bypass opt-in and trusted-proxy configuration
  when enabled, while keeping exported config and middleware values comparable
  for v2 source compatibility.
- `github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics` now keeps the
  existing `NewPrometheusRecorder` signature for v2 source compatibility and
  adds `NewPrometheusRecorderChecked` for callers that want collector
  registration conflicts returned as errors.
- Idempotency mixed-version compatibility metrics now expose only bounded
  `method`, `store_class`, and `outcome` labels. Raw paths, idempotency keys,
  key hashes, and error strings remain available only on structured events for
  logs or traces.
- `middleware/timeout.NewHard` now enforces a bounded response capture size with
  a 1 MiB default. Oversized captured responses return Problem Details instead
  of silently truncating successful responses.
- Admin endpoint docs now steer new pprof and detailed-health mounts toward
  fail-closed registration helpers while preserving legacy source-compatible
  helpers for v2 callers.
- `endpoints/health.Handler.RegisterPublicRoutesTo` and
  `contrib/bootstrap.MountSystemEndpointsToWithAdmin` now give new system
  endpoint wiring a source-compatible path that keeps public probes separate
  from admin-only detailed health, metrics, and pprof routes.
- `webhooks.Receiver` now returns a generic verifier failure detail by default
  so custom verifier errors are not echoed to clients. Use
  `ReceiverConfig.VerificationErrorDetail` only for explicitly safe text.

### Upgrade notes

- If you treated maintained contrib middleware or adapters as semver-stable,
  review `API_BASE_REF=v2.1.0 GOTOOLCHAIN=local make contrib-api-drift-report`
  before upgrading and check `docs/contrib-api-drift-dispositions.tsv` for the
  current package-tied disposition. Contrib drift remains report-only; this
  guidance helps migration review but does not extend the stable v2 API promise
  to contrib.
- For `github.com/aatuh/api-toolkit/contrib/v2/middleware/auth/devheaders`,
  set `AllowDangerousDevBypasses` and `TrustedProxies` explicitly when enabling
  debug-header auth. `TrustedProxies` is a comma-separated CIDR list.
- V3 preparation guidance is now consolidated in
  `docs/v3-compatibility-roadmap.md`: use `compat/billing` or app-owned billing
  ports, database stats snapshots, `httpx` or package-local response helpers,
  and token-aware idempotency release before the major-version compatibility
  removals.

## 2026-05-01

- Release evidence now writes `release-check-summary.json` schema v2 with
  per-check command lines, exit codes, durations, log paths, tool versions, and
  local-vs-GitHub artifact tier metadata.
- `make release-evidence` now runs the release-readiness subchecks through the
  evidence writer so local summaries have detailed provenance instead of a
  fixed pass list.
- `docs/package-classification.tsv` now documents API and test quality tiers,
  and docscheck mechanically validates direct-test, wrapper-smoke, example,
  generated, tooling, test-support, excluded, and needs-tests classifications.
- `docs/v3-compatibility-roadmap.md` now contains one removal matrix for
  provider-shaped billing ports, pgx-shaped database stats, `response_writer`,
  tokenless idempotency release, unchecked authz construction, and checked list
  parser shims.
- `make contrib-api-drift-report` adds a report-only API drift signal for
  selected high-use contrib adapters and integrations without changing the
  contrib compatibility policy.
- `make contrib-release-notes-check` adds a lightweight review gate requiring
  release-note coverage when contrib adapter or integration behavior files
  change.
- Release evidence now records `git_state` with branch/detached state, dirty
  flag, staged/unstaged/untracked/deleted counts, and the commit checked.
- Release evidence now records top-level `publication_eligible`; automation must
  require it to be `true` along with passed status, clean provenance, and clean
  git state before accepting publication evidence.
- Release evidence now fails publication mode on dirty worktrees unless
  `ALLOW_DIRTY_RELEASE_EVIDENCE=1` explicitly marks the output as local
  dirty-tree audit evidence.
- Release evidence now archives `.ci-result/release-evidence/logs` as
  `.ci-result/release-evidence/release-evidence-logs.tgz` and records
  `publication_artifact_expectations` for draft-release asset review.
- `make release-artifact-verify` now verifies downloaded draft release asset
  names, `release-asset-manifest.tsv` checksums, retained release logs, SBOM
  signatures/certificates, and expected provenance subjects before publishing.
- The tag-driven release workflow now verifies keyless SBOM signatures against
  the GitHub OIDC certificate identity and issuer before uploading draft
  release assets.
- Release evidence now records `vulnerability_evidence` from govulncheck logs
  so imported-but-not-called vulnerability IDs and counts have reviewer
  disposition in `docs/dependency-risk.md` and
  `docs/vulnerability-dispositions.tsv`.
- Release evidence now dynamically compares imported-only vulnerability IDs
  with `docs/vulnerability-dispositions.tsv` and fails when dispositions are
  missing, incomplete, or expired on the release review date.
- `make release-evidence` now archives report-only contrib drift at
  `.ci-result/release-evidence/logs/contrib-api-drift-report.log` and summarizes
  drift, skipped, compatible, and incompatible counts in
  `release-check-summary.json`.
- Release evidence now records current contrib drift packages and status,
  compares them with `docs/contrib-api-drift-dispositions.tsv`, and fails when
  current drift has missing or expired disposition coverage.
- Current contrib drift disposition is recorded in
  `docs/contrib-api-drift-dispositions.tsv`, including the incompatible
  report-only `contrib/middleware/auth/devheaders` drift.
- `make release-check` and `make release-evidence` now include
  `contrib-release-notes-check`, while `contrib-api-drift-report` reads selected
  report-only packages from `docs/contrib-api-drift-packages.txt`.
- Incompatible report-only contrib drift is acknowledged for this release:
  `contrib/middleware/auth/devheaders` changed exported struct comparability
  because middleware/config types now include non-comparable lifecycle fields.
  This remains a review signal and does not make contrib stable.
- `make contrib-release-notes-check` now requires incompatible report-only
  contrib drift acknowledgement to mention the affected package, not only a
  generic incompatible-contrib phrase.
- Docscheck now blocks new production source usage of deprecated billing ports
  outside `ports`/`compat/billing` and direct database-stat usage outside
  compatibility or adapter paths.
- Idempotency middleware response capture now uses a package-local helper
  instead of importing the legacy `response_writer` compatibility package.
- `docs/release-review.md` gives release reviewers a shorter path through the
  runbook, release notes, stability policy, package classification,
  compatibility roadmap, and evidence artifacts.
- Wrapper and example coverage policy now distinguishes wrapper smoke minimums
  from build-smoke-only example coverage.
- Package docs for `ports`, `compat/billing`, and the legacy response helper
  package now identify v2 compatibility-sensitive surfaces and preferred
  replacements for new code.
- `contrib/middleware/requestlog` expands header redaction defaults for broader
  authentication/session families and adds payload field redaction helpers for
  non-header custom fields.
- `contrib/adapters/httpclient` retry defaults are now conservative: `GET` and
  `HEAD` only; other methods such as `PUT` and `DELETE` now require explicit
  opt-in through `RetryableMethods`.
- `contrib/bootstrap` pprof mounting now defaults to opt-in behavior and requires
  explicit profile intent to enable `Pprof` routes in production-like defaults.
- Idempotency in-flight reservations now carry `ReservationToken` and require
  tokenized releases for healthy non-legacy records, while legacy tokenless
  records are recovered during mixed-version rollouts when stale past
  `InFlightTTL`.
- Idempotency memory and Redis adapters now expose optional legacy-recovery
  telemetry callbacks for tokenless record migrations (`legacy_in_flight_recovered`
  and `legacy_in_flight_token_mismatch`).
- Idempotency middleware now emits compatibility telemetry for mixed-version
  fallback attempts (`legacy_in_flight_fallback_entered`,
  `legacy_in_flight_fallback_recovered`,
  `legacy_in_flight_fallback_rejected`,
  `legacy_in_flight_fallback_unknown`) and validates cross-service
  `InFlightTTL` alignment via `KnownInFlightTTLs`/`FailOnInFlightTTLMismatch`.
- `ports.ErrLegacyInFlightReservationMissingToken` has been added for migration-time
  observability of legacy in-flight record recovery.
- `middleware/auth/authz` keeps the v2-compatible single-return constructor and
  adds `NewRequireRoleMiddlewareChecked` plus bootstrap validation for explicit
  role requirements and nil resolver detection at route setup.
- `endpoints/list` keeps the v2-compatible single-return parser helpers and adds
  checked variants (`ParseListQueryChecked`, `DefaultFilterParserChecked`,
  `DefaultSortParserChecked`) for callers that need field-level validation
  errors.
- `contrib/middleware/requestlog` documents and supports deep payload redaction for
  common typed container shapes (`map[string]string`, `[]map[string]string`) while
  preserving legacy shallow behavior.
- Idempotency middleware now emits mixed-version fallback telemetry by default when no
  `OnLegacyInFlightCompatibility` callback is configured, defaults legacy
  compatibility keys to stable SHA-256 redaction, and supports explicit raw-key
  opt-in through `LegacyInFlightCompatibilityRawKey`.
- Idempotency startup rollout governance now includes optional strict clock-preflight
  checks (`FailOnInFlightClockSkewPreflight`) for mixed-version safety, emitting
  `ErrLegacyInFlightClockSkewPreflightRisk` in strict mode and advisory
  deprecation-risk warnings in default mode.
- `contrib/adapters/chi` now ships a route bootstrap helper that maps chi route
  registration context into authz role specs and validates role coverage in one
  startup call, including actionable `ANY`/method route context.
- `contrib/middleware/requestlog` normalizes panic observability by always logging
  recovered panics at error level with failure classification, including committed-
  response panics and preserving committed status for optional downstream analytics.
- Release readiness now has a fail-closed `make release-check` path that requires
  `API_BASE_REF=v2.1.0`, keeps local `make api-check` fallback behavior separate,
  and publishes `release-check-summary.json` with release SBOM assets.
- Root idempotency adapter contract coverage moved to contrib-owned reusable
  contract tests so root `go.mod` no longer carries contrib, Redis, or miniredis
  requirements for core middleware tests.

### Upgrade notes

- If your system endpoint wiring relied on implicit pprof exposure in production
  profiles, use an explicit profile-aware mount helper to re-enable it intentionally.
- If you require retries for non-idempotent methods, add them explicitly to
  `RetryableMethods` and confirm the target API contract is idempotent.
- If you are rolling out idempotency migration with shared Redis across mixed
  binary versions, ensure all services agree on `InFlightTTL`. Legacy tokenless
  in-flight entries will be auto-cleared only when stale, and mixed-version
  cleanup can be delayed by that TTL when no newer version processes the key
  first.
- Legacy idempotency cleanup requires aligned timing: set `InFlightTTL` consistently
  across services and storage layers, including matching `InFlightTTL` and key TTL
  behavior, keep `SystemClock` sources synchronized, and ensure record
  `CreatedAt` monotonic assumptions match your deploy latency.
  Checklist during rollout: (1) run `ValidateRequireRoleMiddleware`-style startup
  checks for route wiring on all roles-protected endpoints, (2) verify
  `InFlightTTL` parity and shared store key prefixes across all deploy units,
  (3) monitor middleware telemetry outcomes (`legacy_in_flight_fallback_entered`,
  `..._recovered`, `..._rejected`, `..._unknown`) while mixed binaries run,
  and (4) remove tokenless-compatibility behavior only after mixed-version
  fallback suppression reaches zero.
- Recommended rollout telemetry contract:
  - Labels: `method`, `path`, `store_type`, `outcome`, `key` (optional), and
    `error`.
  - For metric collectors, prefer `LegacyInFlightCompatibilityMetricSink` and
    `LegacyInFlightCompatibilitySampleEvery` during large rollout waves.
  - Use `LegacyInFlightCompatibilityAsync` when callback latency must not affect
    request latency. Keep `Logger`/compatibility sink diagnostics during initial
    rollout windows for deterministic evidence.
  - Default warning thresholds:
    - `legacy_in_flight_fallback_unknown > 0` for 5 minutes indicates release risk
      and should page.
    - `legacy_in_flight_fallback_rejected / legacy_in_flight_fallback_entered` above
      `0.5%` over 10 minutes indicates high key-level contention and should be
      investigated.
    - `legacy_in_flight_fallback_recovered / fallback_entered` dropping below
      `99%` indicates likely TTL/clock contract mismatch.
  - Dashboard query examples:
  ```promql
  sum by (store_type) (rate(legacy_in_flight_fallback_unknown[5m])) > 0
  sum by (store_type) (
    rate(legacy_in_flight_fallback_rejected[10m])
  )
  /
  sum by (store_type) (rate(legacy_in_flight_fallback_entered[10m]))
    > 0.005
  sum by (store_type) (
    rate(legacy_in_flight_fallback_recovered[10m])
  )
  /
  sum by (store_type) (rate(legacy_in_flight_fallback_entered[10m]))
  < 0.99
  ```
- Backpressure behavior:
  - Synchronous sinks execute in the request path; if a custom sink is slow or
    blocking, requests can back up at startup and during mixed-version load.
  - Enable async emission for high-volume migrations and confirm callback
    exceptions are tracked by tests or sink-specific observability, since they are
    intentionally recovered and must not abort request handling.
- If you rely on zero-config retry behavior in `contrib/adapters/httpclient`, review
  all non-GET/HEAD consumers and update to explicit `RetryableMethods` only for
  confirmed replay-safe routes and clients.
- `NewRequireRoleMiddleware` keeps the v2-compatible single-return constructor.
  Invalid wiring is still fail-closed at runtime until fixed and will return
  `401` (no actor) or `403` (actor without role) as applicable. For startup
  validation, use `NewRequireRoleMiddlewareChecked` or run
  `ValidateRequireRoleMiddleware(method, route, mw)` for each protected route.
  ```go
  authzMw, err := authz.NewRequireRoleMiddlewareChecked("admin", func(ctx context.Context) []string {
      if actor, ok := authorization.ActorFromContext(ctx); ok {
          return actor.Roles
      }
      return nil
  })
  if err != nil { panic(err) }
  if err := authz.ValidateRequireRoleMiddleware(http.MethodGet, "/admin", authzMw); err != nil {
      return fmt.Errorf("route contract check failed: %w", err)
  }
  ```
- If you need startup authz migration validation, prefer registry-level validation
  via `ValidateRequireRoleMiddlewareRoutes` during bootstrap and fail startup on
  the first startup pass when any route fails this check.
  ```go
  checks := []authz.RequireRoleRouteSpec{
      {Method: http.MethodGet, Route: "/admin", Middleware: adminMw},
      {Method: http.MethodPost, Route: "/billing", Middleware: billingMw},
  }
  if err := authz.ValidateRequireRoleMiddlewareRoutes(checks); err != nil {
      return fmt.Errorf("route contract scan failed: %w", err)
  }
  ```
- Rollout symptoms of misconfigured authz route wiring are usually a startup
  failure in CI or process init (`invalid role middleware for route ...`), then
  runtime `401` for unauthenticated requests and `403` for missing-role users.
  Rollback sequence when migration checks block startup:
  (1) restore previous middleware wiring,
  (2) temporarily disable strict constructor checks only as a temporary guardrail,
  (3) reapply the startup check after role/route registration is repaired, and
  (4) rerun staged rollout.
- `requestlog` payload redaction assumes redaction-sensitive names based on
  canonical field patterns (`token`, `secret`, `password`, common aliases) before
  any deep traversal. For typed payloads, normalize unsupported custom shapes to
  map/slice-of-map shapes before calling
  `requestlog.RedactPayloadFieldsDeep` (see `contrib/middleware/requestlog/doc.go`).

- For mixed-version idempotency rollouts, run startup with `FailOnInFlightTTLMismatch`
  and `FailOnInFlightClockSkewPreflight` only after you have parity checks and
  rollback strategy in place. Keep both off during the first boot of a migration
  wave if you need warning-only discovery.
- `ports.IdempotencyReleaser.Release(ctx, key)` remains the v2 compatibility
  contract for existing custom stores. New stores should also implement
  `ports.IdempotencyReservationReleaser.ReleaseReservation(ctx, key, token)` so
  middleware can release only the current tokened in-flight reservation.
- To upgrade authz checks with chi, either build explicit `[]authz.RequireRoleRouteSpec`
  and validate via `authz.ValidateRequireRoleMiddlewareRoutes` or use
  `chi.ValidateRequireRoleMiddlewareRoutes` with a route+method resolver closure to
  map protected handlers.

## 2026-04-24

- `contrib/telemetry.WrapHTTPClient(nil)` now creates an instrumented client with a 10 second timeout instead of an unbounded zero-timeout client.
- `contrib/migrator.Options.LockTimeout` can now override the advisory lock wait timeout; zero keeps the previous 10 minute default.
- `contrib/migrator.Options.UnlockFailureHandler` and the existing migrator logger can now surface advisory unlock failures without replacing the primary migration result.

### Upgrade notes

- If you intentionally need no client-level timeout for a telemetry-wrapped `net/http` client, pass an explicit `&http.Client{}` to `WrapHTTPClient`; prefer request contexts with deadlines for long-running calls.

## 2026-04-23

- Billing contracts in `ports/billing.go` are now formally deprecated for new code. The same Stripe-shaped v2 model is available through the new compatibility package `github.com/aatuh/api-toolkit/v2/compat/billing`.
- `contrib/adapters/pgxpool.Adapter.StatSnapshot()` now copies plain-value pool stats directly from pgxpool instead of routing through the legacy `DatabaseStats` wrapper path.

### Upgrade notes

- Existing code that imports billing contracts from `ports` keeps working for the rest of v2, but new code should migrate to `github.com/aatuh/api-toolkit/v2/compat/billing` so the provider-shaped dependency is explicit before v3 extraction.
- If your health or observability code still reads `DatabasePool.Stat()` or depends on `DatabaseStats`, move it to `DatabasePoolSnapshotProvider`, `SnapshotDatabasePoolStats`, or adapter `StatSnapshot()` methods. The legacy counter interface remains for compatibility adapters, not as the preferred generic path.

## 2026-04-19

- `contrib/middleware/auth/devheaders` now requires explicit dangerous-bypass opt-in and trusted-proxy configuration before it will honor debug auth headers.
- Health endpoints now fail closed on empty or miswired liveness/readiness probe sets, and HTTP handlers only expose detailed dependency output when `ports.HealthCheckConfig.EnableDetailed` is explicitly enabled.
- `contrib/adapters/txpostgres.WithinTx` now attempts deferred rollback with a bounded cleanup context even when the caller context is already canceled or timed out.
- `contrib/adapters/txpostgres` now fails closed with `ErrPoolNotConfigured` when callers forget to wire a database pool, instead of panicking on nil-pool use.
- `endpoints/docs.New()` and `NewDefaultHandler()` now default to the first-party static docs surface; callers must opt into the CDN-backed Swagger UI mode with `docs.NewSwaggerUI()` or `DocsConfig.HTMLMode`.
- `contrib/migrator` now records commit-acknowledgement failures as `uncertain` and blocks later runs when a prior migration record is still `started` or `uncertain`.
- `scheduler.Runner` now persists final run records through a bounded cleanup context so graceful shutdown does not drop `LastFinished` updates for jobs that already completed.
- `scheduler.Runner` now surfaces recorder persistence failures through structured logs and optional `SetRecorderFailureHandler` callbacks without changing the completed job result or schedule cadence.
- JWT and Clerk middleware now share internal auth/JWKS validation primitives with no intended public API or configuration change.

### Upgrade notes

- If you previously enabled `devheaders` without explicitly opting into dangerous bypasses or without trusted-proxy configuration, startup will now fail fast until you set both intentionally.
- If you had tests or thin wiring paths that called `txpostgres.New(nil)` or `txpostgres.FromCtx(..., nil)`, they now return `ErrPoolNotConfigured` instead of panicking.
- If you relied on `docs.New()` or `NewDefaultHandler()` to serve Swagger UI with CDN assets, switch to `docs.NewSwaggerUI()` or set `DocsConfig.HTMLMode = ports.DocsHTMLModeSwaggerUI` explicitly.
- If a deployment previously canceled scheduler job contexts during graceful shutdown, completed jobs now get a short recorder-persistence window before exit so restart-time suppression remains accurate.
- If operators previously relied on `/health` or equivalent routes exposing dependency-level detail by default, set `EnableDetailed` explicitly during wiring; otherwise only basic probes should remain visible.
- If your deployment workflow retried migrations automatically after commit errors, stop doing that. Inspect the database state and reconcile `schema_migrations` before rerunning when a migration is recorded as `started` or `uncertain`.
- If you need alerting when scheduler run history cannot be persisted, wire `SetRecorderFailureHandler` or monitor the new recorder-failure log events; job completion alone no longer implies recorder persistence succeeded.
- JWT and Clerk integrations should be behaviorally equivalent to their prior public APIs, but custom wrappers that depended on edge-case differences in bearer parsing, claim requirements, or skip-header handling should be revalidated.

## 2026-04-15

- Idempotency middleware now releases failed reservations after downstream `5xx` responses and panics, so retries with the same payload and `Idempotency-Key` are not blocked behind a stale in-flight record.
- Idempotency middleware now fails closed with `503 Service Unavailable` when it cannot persist a completed replay record, and it stores an ambiguous state for that key instead of reopening it for another execution.
- Idempotency middleware now includes authenticated actor and tenant scope in the default request hash, preventing cross-principal or cross-tenant replays from reusing the same key and payload.
- Idempotency middleware now caps buffered replay bodies at `1 MiB` by default and returns `503 Service Unavailable` plus an ambiguous key state when a handled response exceeds the replay buffer limit.
- `scheduler.Runner` now recovers scheduled-job panics, logs and records them as failed runs, and keeps future intervals alive instead of letting one bad job crash the process.
- `scheduler.Runner` now prevents the same job name from overlapping with itself across duplicate `Start` calls or duplicate scheduling of the same job.
- `bootstrap.ProfileStrictAPI` no longer enables wildcard CORS by default; browser-facing cross-origin access now requires an explicit `WithCORSOptions(...)` allowlist.
- `contrib/config.LoadFromEnv` now treats invalid present bool and int values as startup errors instead of silently falling back to defaults.
- Docs endpoints now return `404` when the HTML docs surface is disabled or when no authoritative OpenAPI document is available.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats may be served on the configured docs path.
- Multi-source migrator loading now documents its actual contract: duplicate version+direction pairs are rejected.
- The pagination example now returns one field-level validation shape for invalid `limit` inputs even when `querylimits` rejects the request before the handler.

### Upgrade notes

- If clients previously saw `409 Conflict` after a failed idempotent write, retry behavior has changed: the same payload and `Idempotency-Key` can now be retried immediately after downstream `5xx` and panic paths, but not after completed-response persistence failures or replay-buffer overflows.
- If clients previously received the original success response even though completion persistence failed, they now receive `503 Service Unavailable` and the key remains blocked in an ambiguous state until it expires or is reconciled.
- If authenticated middleware previously ran after idempotency, default caller scoping will not apply. Move auth and tenant middleware earlier in the stack to keep replay protection scoped per caller.
- If a route can stream, hijack, upgrade, or return large bodies, exclude it with `ShouldHandle` or raise `MaxResponseBytes`; otherwise oversized handled responses now fail closed with `503 Service Unavailable` and block same-key retries for the key lifetime.
- If a scheduled job panic previously terminated the process, that failure is now contained and surfaced through scheduler logging and run recording instead.
- If application code called `scheduler.Runner.Start` more than once or reused the same job name across duplicate schedules, those executions no longer overlap. Validate any workload that previously relied on concurrent execution of the same named job.
- If browser clients previously relied on `ProfileStrictAPI` to emit `Access-Control-Allow-Origin: *`, they must now set an explicit allowlist with `WithCORSOptions(...)` during bootstrap.
- If deployment environments previously contained malformed bool or int values such as `MIGRATE_ON_START=maybe`, startup now fails fast instead of silently using defaults. Validate env files and secrets before rollout.
- If deployment environments used undocumented semantic values such as `ENV=qa`, `ENV=prod`, `LOG_LEVEL=verbose`, or `LOG_LEVEL=warning`, startup now fails fast. Use `development|staging|production` for `ENV` and `debug|info|warn|error` for `LOG_LEVEL`.
- Docs handlers no longer return a synthetic OpenAPI document when no authoritative spec exists. Expect `404` for disabled docs surfaces and for missing OpenAPI files unless a real document is configured.
- `DocsConfig.EnableJSON` and `DocsConfig.EnableYAML` now control which discovered OpenAPI formats can be served. Verify custom docs paths and any YAML-based docs setup during upgrade.
