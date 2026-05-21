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
- Run release evidence through the runbook path; `docs/release-runbook.md`
  owns the current supported `API_BASE_REF` baseline and exact commands, while
  `make finalize` and `make audit-check` are local/reviewer gates.
- Run `make contrib-api-drift-report` with the same release baseline when
  selected contrib adapters or integrations change exported APIs; selected
  packages come from `docs/contrib-api-drift-packages.txt`,
  supported-adapter incompatible drift is gate-enforced, and this does not make
  contrib stable.
- Run `make contrib-release-notes-check` with the same release baseline when
  supported contrib adapter, integration, middleware, bootstrap, telemetry,
  production generator CLI behavior files, or runtime assets change.
- Supported-adapter contrib packages remain outside the stable core API promise, but incompatible public API drift in that tier must be treated as gate-enforced and resolved with compatibility, reclassification, or a major-release policy decision.
- If there is incompatible report-only contrib drift, add an explicit release note or upgrade note acknowledgement tied to the affected package. This does not make contrib stable.
- Update `docs/vulnerability-dispositions.tsv` when imported-only vulnerability IDs change, expire, or receive upgraded dependencies.
- Update `docs/contrib-api-drift-dispositions.tsv` when current contrib drift packages or incompatible drift status changes.
- Use clean publication evidence with the explicit baseline command from
  `docs/release-runbook.md`; reserve `ALLOW_DIRTY_RELEASE_EVIDENCE=1` for
  local dirty-tree audit evidence that is not acceptable before publishing.
  First v3 major-release evidence may use `API_BASE_REF=v2.1.0` only as
  documented v2-to-v3 transition evidence.
- Use `docs/release-manifests.md` when interpreting `docs/package-classification.tsv`, `docs/contrib-api-drift-dispositions.tsv`, and `docs/vulnerability-dispositions.tsv`.

## v3 cleanup branch

### Breaking cleanup

- The module paths are now `github.com/aatuh/api-toolkit/v3` and `github.com/aatuh/api-toolkit/contrib/v3`.
- Provider-shaped billing exports were removed from root `ports`; use `github.com/aatuh/api-toolkit/v3/compat/billing` for the hosted-checkout compatibility model or define app-owned billing ports.
- `ports.DatabasePool.Stat`, `ports.DatabaseStats`, `ports.SnapshotDatabaseStats`, and the public `response_writer` package were removed. Use `ports.DatabasePoolSnapshotProvider`, `ports.SnapshotDatabasePoolStats`, adapter `StatSnapshot()` methods, and `httpx`.
- Idempotency middleware now requires token-aware release through `ports.IdempotencyReservationReleaser`.
- `authz.NewRequireRoleMiddleware` now validates at construction time and returns `(*RequireRoleMiddleware, error)`.
- List endpoint helpers keep the checked parser APIs: `ParseListQueryChecked`, `DefaultFilterParserChecked`, and `DefaultSortParserChecked`.

## 2026-05-20

### End-game hardening

- `make generated-upgrade-compat-check` now accepts
  `GENERATED_UPGRADE_COMPAT_REFS` and defaults to checking both `v3.0.0` and
  `v3.1.1`; `GENERATOR_REF` remains as a source-compatible single-ref alias.
- Generated upgrade compatibility evidence now writes one log per generator ref
  plus `.ci-result/generated-upgrade-compat/status.tsv`.
- Full-profile resource generation tests now prove the generated `project`
  replacement path with required/default/enum fields, filters, deterministic
  sorts, OpenAPI/client checks, contract checks, and `resource-check` evidence.
- Full-profile docs and generated READMEs now state that sample `widgets` are
  app-owned starter domain code meant to be replaced or complemented by product
  resources.
- Added `make reference-service-evidence`, which records non-blocking
  reference-service proof under `.ci-result/reference-service/`, with optional
  `REFERENCE_SERVICE_DOCKER=1` and `REFERENCE_SERVICE_MINIO=1` runtime evidence.
- Added a reference-service adoption evidence template for setup time, upgrade
  results, OpenAPI/client checks, tenant isolation, idempotency, backup/restore,
  load-smoke notes, and known pain points.

### Release proof and reference service

- Removed the temporary `.next_steps.md` release checklist after publishing
  `v3.1.0`; future release baseline guidance now lives in the release runbook.
- Added `examples/reference-saas-api` as a checked-in `saas-api-full` adoption
  proof service with local workspace replacements, typed client output,
  OpenAPI/contract assets, Docker integration assets, deployment starters, and
  observability assets.
- Added `make reference-service-check` as optional non-Docker evidence for the
  checked-in reference service. It stays outside default `finalize`.
- Generated `saas-api-full` `.gitignore` files no longer ignore
  `internal/client/apiclient`, so the checked-in typed Go client can be tracked
  by generated services.

### Contrib validation adapter

- `contrib/adapters/validation` now uses
  `github.com/aatuh/validate/v3@v3.0.7` instead of
  `github.com/go-playground/validator/v10`.
- Validation tags in toolkit examples now use the validate v3 grammar, such as
  `validate:"string;required;email"` and `validate:"int;min=1"`.
- Field errors now preserve validate v3 JSON field paths and stable error codes
  while continuing to avoid raw submitted values in error strings. The
  deprecated `ValidationError.Value` field is retained for source compatibility
  but is no longer populated by the adapter.
- `NewPlaygroundValidator` remains as a deprecated source-compatible alias, but
  it no longer constructs a go-playground-backed validator. Use
  `NewValidateValidator` for new code.

## 2026-05-19

### Maturity evidence

- Added `make generated-upgrade-compat-check`, an opt-in generated-service
  upgrade compatibility signal that generates `saas-api-full` from the prior v3
  baseline, replaces toolkit dependencies with the workspace, and runs generated
  tests, OpenAPI, client, and contract checks. This stays outside `finalize`.
- Raised the JWT middleware package coverage floor after adding behavior tests
  for valid subject propagation, skip-header enforcement, nil/disabled handler
  behavior, safe close behavior, and JWKS health checks.
- Raised the health endpoints package coverage floor after adding behavior
  tests for public liveness/readiness separation, dependency state transitions,
  timeout mapping, public detail redaction, admin-only detailed health access,
  dependency checker options, and scheduler callbacks.
- Raised the OpenAPI validation middleware coverage floor after adding behavior
  tests for option constructors, OpenAPI file loading, route failure Problem
  Details, request validation field mapping, response validation error hooks,
  streaming opt-outs, large-response bypasses, and response buffering limits.
- Raised webhook delivery and Postgres webhook delivery adapter coverage floors
  after adding behavior tests for signing, endpoint policy, retry
  classification, safe error surfaces, tenant mismatch rejection, replay
  safety, attempt recording, secret resolution, and readiness health.
- Raised the pgxpool adapter coverage floor after adding behavior tests for
  constructor validation, bounded startup contexts, database readiness mapping,
  plain-value snapshots, legacy stats wrappers, acquire failures, and close
  idempotence.
- Added a docscheck gate that every `supported-adapter` contrib package has
  direct tests, package docs, a behavior-contract row, and release drift
  coverage before it can retain the supported-adapter classification.
- Added a manifest-driven adapter maturity review to the production-readiness
  docs so supported adapters are visible as evidence-complete and experimental
  packages are clearly not promoted.
- Updated the release workflow provenance attestation action from the older
  `actions/attest-build-provenance` generation to a pinned v4.1.0 commit while
  preserving release artifact verification semantics.
- Updated generated lean and full scaffold GitHub Actions templates to pinned
  `actions/checkout` v6.0.2 and `actions/setup-go` v6.4.0 commits.
- Added `make actions-audit` and contract coverage for pinned GitHub Actions
  workflow refs, stale action comments, and generated workflow template versions;
  it runs in `make audit-check` and remains non-mutating.
- Tightened README and production-readiness positioning so api-toolkit is
  explicitly scoped to conventional HTTP/JSON API infrastructure, not a
  universal backend platform, and generated code is app-owned.
- Aligned the release runbook with end-game proof targets by making
  `actions-audit`, `coverage-check`, generated upgrade compatibility, generated
  integration, and reference-service evidence visible to release reviewers while
  keeping Docker-backed checks opt-in.
- Tightened the optional GitHub governance verifier so release tag protection
  covers both root `v*` tags and contrib module `contrib/v*` tags.
- Removed the local root-module `replace` directive from `contrib/go.mod` so
  the contrib CLI can be installed with
  `go run github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit@vX.Y.Z`.
- Added `docs/coverage-hardening-backlog.md` to make the next JWT, health,
  pgxpool, OpenAPI middleware, and webhook delivery coverage floor increases
  conditional on behavior-test evidence rather than numeric threshold churn.
- Raised maturity evidence for high-risk v3 surfaces with additional JWT,
  OpenAPI validation, and bootstrap tests. The package coverage gate now keeps
  the OpenAPI validation middleware and bootstrap floors aligned with the new
  observed coverage.
- Promoted production-relevant contrib packages to `supported-adapter` after
  direct tests, package docs, behavior-contract rows, and drift coverage were
  confirmed: `contrib/adapters/httpclient`, `contrib/adapters/envvar`,
  `contrib/config`, `contrib/adapters/validation`,
  `contrib/adapters/migrate`, `contrib/migrator`, and
  `contrib/scheduler/postgres`.
- OPA and Cedar policy adapters now use a shared policy-engine contract for
  provider-neutral request mapping, allow/deny decisions, malformed input
  failures, and safe error surfaces, and are promoted to `supported-adapter`.

### Upgrade notes

- Contrib packages promoted to `supported-adapter` remain outside the stable
  root SemVer promise. Incompatible supported-adapter drift is now
  release-gated and must be release-noted.

## 2026-05-02

### Correctness, security, and release governance

- `docs/full-service-scaffold.md` now defines the planned `saas-api-full`
  production profile contract, including Postgres + Redis defaults, tenant
  resources, durable async/outbox behavior, audit events, webhook delivery,
  OpenAPI 3.1, typed Go client output, opt-in Docker integration checks, and
  base Kubernetes assets.
- `scripts/contrib_release_notes_check.sh` and its contract tests now require
  release-note coverage for future `saas-api-full` full-profile runtime assets
  under `contrib/cmd/api-toolkit`, including generated Kubernetes YAML and
  other scaffold templates.
- `api-toolkit new service` now supports an initial `--profile saas-api-full`
  scaffold with API-key auth, hexagonal `internal/domain`, `internal/app`,
  `internal/adapters/postgres`, and `internal/httpapi` boundaries, Postgres
  migrations for tenant/platform tables, Docker Compose Postgres/Redis assets
  with optional MinIO, Kubernetes starter manifests, OpenAPI golden checks,
  contract lint/diff/client-check targets, checked-in Go client output, and
  generated HTTP smoke tests for readiness, OpenAPI, auth failure, validation
  failure, idempotent create replay, and ETag conflicts.
- `api-toolkit new service --profile saas-api-full` now accepts repeatable
  `--with stripe-billing|resend-email|clerk-webhooks` flags. Selected provider
  workflows generate app-owned `internal/providers` starter packages, provider
  docs, env examples, manifest entries, fake-provider tests, tenant-scoped
  audit behavior, and webhook/signature verification boundaries without adding
  provider-specific imports to the toolkit root module.
- The async, audit, cache, objectstore, webhookdelivery, OIDC middleware, OIDC
  integration, and their Postgres/Redis/S3 adapters now have supported-adapter
  classification, package contract rows, drift-gate coverage, and release-note
  requirements. Postgres audit, operation, outbox, and webhook delivery stores
  also expose readiness health checkers, and `contrib/async/asynctest` adds a
  reusable async store contract suite for adapter implementations.
- `api-toolkit --help`, `api-toolkit -h`, `api-toolkit help`, and equivalent
  subcommand help forms now return usage with exit code `0`; unknown commands
  continue to exit `2`.
- `api-toolkit clients typescript --style fetch` now generates a browser/stdlib
  `fetch` TypeScript package for the same supported OpenAPI subset as the typed
  Go client: JSON bodies, path/query/header params, API-key and bearer auth,
  Problem Details errors, nullable fields, enums, and raw response access.
  `api-toolkit new service --profile saas-api-full --client typescript` adds the
  checked-in TypeScript client package and `client-ts-check` target while keeping
  the existing generated Go client path source-compatible. Generated TypeScript
  configs include DOM iterable fetch types, and `client-ts-check` runs a local
  TypeScript build when `node_modules` is already present.
- `api-toolkit ops observability --profile saas-api-full` now emits a bounded
  label Grafana/Prometheus/runbook bundle for the full scaffold, and
  `api-toolkit deploy helm` plus `api-toolkit deploy terraform --cloud aws`
  generate deployment starters for API, worker, migration, admin service,
  dependency references, and AWS Postgres/Redis/S3 primitives. Generated full
  services now include `cmd/assetcheck` plus `make observability-check`,
  `make deploy-check`, and `make asset-check` so those starter assets are
  validated offline without Helm, Terraform, jq, or network access. Release
  evidence now records those generated asset checks in
  `full_profile_scaffold_evidence.asset_validation`.
- Generated `saas-api-full` migrator commands now include `plan`, `verify`, and
  a guarded `down` command. Down migrations require both
  `--allow-dangerous-down` and `ALLOW_DANGEROUS_MIGRATION_DOWN=true`, and remain
  documented as local/schema-teardown only. When both guards are present, the
  generated command now delegates to `bootstrap.RunDown` and reverts one latest
  applied migration through the contrib migrator.
- `api-toolkit generate resource` now accepts the v2 field and route-shaping
  flags `--field`, `--filter`, `--sort`, `--admin`, `--relationship`, and
  `--object-field`, validating the field DSL before mutating generated projects.
  Generated resources now wire exact-match list filters and allow-listed
  deterministic sorts through HTTP query parsing, application services,
  parameterized Postgres queries, OpenAPI parameters, generated typed clients,
  and partial Postgres indexes. Relationship flags add `<name>_id` fields, and
  object-backed fields must end in `_key` and expose only object keys, not
  payloads. `--admin` now mounts a generated admin-list endpoint under
  `/admin/<plural>` on the admin router only, protected by `X-Admin-Key` and an
  explicit tenant selector.
- Provider workflow scaffolds now include `cmd/provider-replay`, and generated
  provider-check runs package tests plus deterministic replay validation
  for checked-in Stripe, Resend, and Clerk fake fixtures. Live provider checks
  remain gated by `RUN_PROVIDER_LIVE_CHECKS=true`.
- `api-toolkit contracts changelog` and `api-toolkit contracts impact` now
  report OpenAPI operation additions/removals and machine-readable breaking
  client impact for release review. Contract lint and impact checks now also
  cover OpenAPI 3.1 composition review metadata, streaming and binary response
  metadata, callback/webhook metadata, schema default changes, enum widening
  and narrowing, and oneOf/anyOf/allOf composition changes.
- `api-toolkit new service --profile saas-web --auth session|oidc-session` now
  emits a separate browser/session starter so API-first profiles stay unchanged.
  The generated profile includes cookie security defaults, memory and Redis
  session-store boundaries, guarded production startup validation, CSRF
  middleware, OIDC callback state validation, browser-safe CORS, and session
  fixation tests without adding session dependencies to the root module.
- `api-toolkit new service --profile saas-api-full --with entitlements` now
  emits provider-neutral generated app code for plans, features, quotas, usage
  counters, OpenAPI entitlement routes, Postgres `tenant_entitlements` and
  `billing_mappings` persistence, and billing-provider composition guidance.
  The workflow composes with `--with stripe-billing` by updating app-owned
  billing mappings before entitlement changes, without adding Stripe-shaped
  ports to core.
- `github.com/aatuh/api-toolkit/contrib/v3/entitlements` now provides
  provider-neutral feature and quota contracts, low-cardinality decisions,
  reusable store contract tests, and HTTP enforcement middleware that avoids
  exposing tenant or billing identifiers in Problem Details responses.
- Release evidence now expands `full_profile_scaffold_evidence` with explicit
  fields for OpenAPI 3.1 full scaffold output, typed client generation, resource
  generator checks, provider-flag generation, worker wiring, generated
  integration workflow assets, and opt-in Docker integration status. The focused
  `full-profile-scaffold-check` target now covers provider workflow generation
  and resource generation in addition to the full scaffold auth modes.
- Generated `saas-api-full` services now include tenant domain and application
  services for organizations, memberships, invitations, role checks, and
  invitation acceptance. The generated service hashes invitation tokens before
  storage, returns the raw invitation token only from the create-invitation use
  case, and includes generated unit tests for owner membership, role failures,
  wrong-token failures, and single-use invitation acceptance.
- Generated `saas-api-full` HTTP routers now expose organization create/list,
  member list, invitation create, and invitation accept routes with OpenAPI
  contracts, generated Go client methods, idempotency metadata, tenant policy
  metadata, and generated HTTP tests for role failures and token replay.
- Generated `saas-api-full` services now include API-key lifecycle management
  for organization-scoped create/list/revoke, scoped permissions, one-time raw
  secret return, non-secret key prefixes, peppered SHA-256 hash storage,
  last-used tracking on verification, and generated OpenAPI/client coverage.
- `api-toolkit new service --profile saas-api-full --auth jwt|clerk|oidc`
  now emits matching bearer-auth runtime wiring, generated auth tests, tenant
  claim checks, scope checks, and BearerAuth OpenAPI security instead of falling
  back to API-key-shaped full-profile router code.
- Generated `saas-api-full` services now include an async widget import
  workflow using `202 Accepted`, `Location`/`Retry-After`, tenant-scoped
  operation polling at `GET /operations/{id}`, replay-safe idempotency, a
  generated worker service over the contrib async store/handler contracts, and
  OpenAPI/client coverage for `createWidgetImport` and `getOperation`.
- Generated `saas-api-full` services now wire optional Postgres runtime
  startup checks: when `DATABASE_URL` is set, generated code opens a pgx pool,
  pings it, verifies required platform tables, closes the pool on shutdown, and
  reflects database failures through public readiness and admin detailed health.
- Generated `saas-api-full` services now use `bootstrap.NewAPIService` as the
  composition root for public/admin listeners, strict SaaS middleware order
  validation, safe system endpoint mounting, graceful shutdown, and async
  worker lifecycle. The full profile now exposes `/livez` separately from
  `/readyz`, keeps liveness process-only, moves detailed health/metrics/pprof
  to the admin listener when `ADMIN_ADDR` is set, and enables runtime OpenAPI
  request validation by default with response validation enabled in
  development/test or by `OPENAPI_RESPONSE_VALIDATION=true`.
- Generated `saas-api-full` services now include an in-process audit recorder
  and write-route hooks for organization, invitation, API-key, widget, and
  async import actions, with generated tests proving audit metadata redaction
  and no raw API-key secret leakage.
- Generated `saas-api-full` services now include outbound webhook event
  catalog, endpoint create/list, delivery list, and delivery replay routes;
  widget writes enqueue tenant-scoped pending deliveries for subscribed
  endpoints, generated OpenAPI/client output covers those operations, and tests
  prove webhook signing secrets are returned only at endpoint creation.
- Generated `saas-api-full` OpenAPI documents now opt into OpenAPI 3.1 through
  `specs.NewRegistryWithOptions(... OpenAPIVersion31)` while the lean
  `saas-api` scaffold keeps the existing OpenAPI 3.0 default.
- Generated `saas-api-full` services now include a generated cache service,
  in-memory local cache store, Redis cache adapter, `CACHE_STORE` configuration,
  cache readiness composition, and cached webhook event catalog responses with
  generated tests for TTL, cloning, Redis address validation, and cache hits.
- Generated `saas-api-full` services now include tenant-scoped object storage
  routes and application services with strict key, content-type, and size
  validation, OpenAPI/client coverage, audit hooks, and tests proving object
  payloads are not exposed in list/create responses or validation problems.
- Release evidence now records `full_profile_scaffold_evidence`, and
  `make release-check` includes a focused `make full-profile-scaffold-check`
  target so the generated `saas-api-full` service, OpenAPI/contract workflow,
  and generated Go client are explicit release signals. Generated Docker
  integration checks remain opt-in and are reported separately through the
  non-blocking integration evidence status.
- Generated `saas-api-full` `integration-check` now uses a dedicated
  script that starts Postgres and Redis, applies the generated migration, runs
  generated unit tests, starts the API on localhost, and performs HTTP smoke
  checks for readiness, OpenAPI, authentication failure, tenant membership,
  managed API-key authentication, idempotent widget writes, ETag conflict
  handling, async operation polling, outbox completion/retry behavior, webhook
  delivery/replay, object write/readback, audit writes, admin detailed health,
  admin metrics, admin pprof, and public admin-route isolation before tearing
  Docker volumes down. Set `INTEGRATION_OBJECT_STORE=s3` to have the script
  start the optional MinIO profile, initialize the generated `api-objects`
  bucket, and run the same object checks through the S3-compatible adapter.
  Fresh generated checkouts now materialize `.env` from `.env.example` before
  invoking Docker Compose, and the generated Postgres volume mount uses the
  PostgreSQL 18-compatible `/var/lib/postgresql` parent directory. Generated
  full-profile Makefile, Dockerfile, and integration checks now hydrate module
  sums with `go mod tidy` before build or test commands, and generated `go.mod`
  files use the installed toolkit release version instead of pinning the stale
  v2.1.0 baseline when the CLI is installed from a SemVer tag. The generated
  integration script now feeds SQL through stdin so psql variables are expanded,
  isolates generated auth tests from integration actor environment variables,
  uses current-compatible MinIO `mc mb --ignore-existing` flags, and tears down
  Compose with the objectstore profile enabled so optional MinIO resources do
  not remain running after S3 checks.
- Postgres audit and outbox adapters now exercise real-SQL failure paths more
  closely: audit SQL no longer includes Go comment text, and outbox retry
  scheduling casts the retry base timestamp before adding interval backoff.
- Generated `saas-api-full` widget services now use an application storage
  port, and the generated runtime switches to a Postgres widget store when
  `DATABASE_URL` is configured. The store persists widget create/update/delete
  state in the generated `widgets` table while preserving the local in-memory
  default for tests and lightweight development.
- Generated `saas-api-full` API-key services now switch to a generated
  Postgres API-key store when `DATABASE_URL` is configured. The store persists
  only keyed hash bytes, display prefixes, scopes, expiry, revocation, and
  last-used timestamps; raw API-key secrets are still returned once and are not
  durable data.
- Generated `saas-api-full` tenancy services now switch to a generated
  Postgres tenancy store when `DATABASE_URL` is configured. The store persists
  organizations, owner memberships, role checks, invitation token hashes,
  invitation acceptance, and member listing while keeping raw invitation tokens
  return-once only.
- Generated `saas-api-full` async widget imports now switch to generated
  Postgres operation/outbox wiring when `DATABASE_URL` is configured. The app
  service writes tenant-scoped pollable operation rows, enqueues outbox work,
  and the generated outbox store leases work through contrib async while
  keeping failure problems sanitized.
- Generated `saas-api-full` Postgres runtimes now route the shared outbox
  through `contrib/async`'s handler mux, dispatching `widgets.import` to the
  widget importer and `webhook.delivery` to the outbound webhook deliverer.
  Webhook attempts are recorded through the generated app/Postgres store
  boundary with sanitized errors and low-cardinality delivery metrics.
- Generated `saas-api-full` services now include a dedicated `cmd/worker`
  binary for background jobs, an `ASYNC_WORKER_ENABLED` switch for API
  processes, Docker Compose worker service wiring, a Kubernetes worker
  Deployment, and integration-check startup that exercises the worker
  separately from the public API process.
- Generated `saas-api-full` integration checks now run a local webhook
  receiver, prove successful outbound delivery and replay reach it, verify
  failing webhook endpoints record retryable delivery state, force a poison
  outbox row into `dead_letter`, and check receiver/delivery output does not
  expose the generated signing secret.
- Generated `saas-api-full` services now emit contrib migrator-compatible
  `*.up.sql` migrations plus a generated `cmd/migrate up|status|check`
  binary. Docker Compose runs a dedicated `/migrate -dir /migrations up`
  service before API/worker startup, the integration script applies and checks
  migrations through `cmd/migrate`, and the Docker image now includes
  `/migrate` plus `/migrations`.
- Generated `saas-api-full` Kubernetes assets now include ConfigMap, Secret
  placeholder, migration Job, worker Deployment, internal-only admin Service,
  PodDisruptionBudget, HPA, NetworkPolicy, resource requests/limits, non-root
  security contexts, and `/livez`/`/readyz` probes. The generated integration
  workflow is opt-in through `workflow_dispatch` and scheduled runs instead of
  default PR CI.
- Generated `saas-api-full` services now include an `api-toolkit.yaml`
  manifest and `resource-check` target, and `api-toolkit generate resource`
  now supports manifest-gated tenant-scoped CRUD generation inside full-profile
  projects. The generator adds domain/app/Postgres/httpapi files, a
  contrib-migrator `*.up.sql` migration, route/OpenAPI contracts, audit hooks,
  webhook event hooks, OpenAPI golden regeneration, typed Go client
  regeneration, and fails closed when expected generated anchors are missing.
- Generated `saas-api-full` object routes now support `OBJECT_STORE=s3` via a
  generated blob-store port and S3-compatible adapter wrapper. Tenant and role
  checks remain in the app service; object bytes are written, read, and deleted
  through the contrib S3 adapter with bounded size and content-type policy.
- Generated `saas-api-full` S3 object routes now use a generated Postgres
  object metadata store when `DATABASE_URL` is configured, so tenant-scoped
  list/get/delete state survives process restarts while payload bytes remain in
  the object store.
- Generated `saas-api-full` webhook routes now switch to a generated Postgres
  webhook store when `DATABASE_URL` is configured. Endpoint signing secrets are
  encrypted with `WEBHOOK_SECRET_KEY`, delivery history is tenant-scoped, and
  replay updates the delivery row while requeueing the matching outbox job.
- Generated `saas-api-full` unsafe write routes now use the core idempotency
  middleware with tenant-aware hashed storage keys. Local scaffolds default to
  in-memory replay; `IDEMPOTENCY_STORE=redis` wires the generated Redis adapter
  for cross-instance replay state.
- Generated `saas-api-full` protected routes now use the core rate-limit
  middleware. Local scaffolds default to in-process buckets; production
  defaults require `RATE_LIMIT_STORE=redis` and wire a generated Redis limiter
  with hashed actor/tenant/route keys.
- Generated `saas-api-full` services now create a contrib Prometheus recorder,
  wrap public `net/http` routes with HTTP metrics middleware, and serve the
  standard Prometheus handler only behind admin authentication. Generated tests
  assert request metrics use route-pattern labels and do not expose tenants,
  actors, API keys, admin keys, or idempotency keys.
- Generated `saas-api-full` admin routers now mount real Go pprof handlers via
  `pprof.RegisterAdminRoutes` instead of returning a placeholder response.
  Generated tests assert pprof is absent from the public handler and requires
  `X-Admin-Key` on the admin handler.
- Generated `saas-api-full` API-key auth mode now verifies generated API keys
  through the generated API-key service when the static bootstrap `API_KEY`
  does not match. Managed keys enforce route scopes, bind requests to their
  organization, update last-used state, fail after revocation, and keep raw key
  secrets out of Problem Details.
- Generated `saas-api-full` audit recording now delegates to the contrib
  Postgres audit store when `DATABASE_URL` is configured, after the generated
  service has produced event IDs, timestamps, and redaction-safe metadata.
  Local development keeps the existing in-memory audit recorder.
- `specs.NewRegistryWithOptions` now supports explicit OpenAPI 3.1 output via
  `specs.RegistryOptions{OpenAPIVersion: specs.OpenAPIVersion31}` while
  preserving the existing `specs.NewRegistry` OpenAPI 3.0 default.
- `specs` now includes additive schema helpers for reusable refs, nullable
  schemas, examples, enum values, struct-tag examples/enums/nullable fields,
  request/response media examples, and reusable HTTP Problem Details response
  components.
- `api-toolkit clients go` now generates a stdlib-only Go client package from
  OpenAPI operations, including operation methods, path/query/header request
  options, JSON request bodies, API-key and bearer auth helpers, and Problem
  Details error decoding.
- `api-toolkit clients go --style typed` now generates component schema
  structs, typed request/response operation methods, typed Problem Details
  error handling, and raw method escape hatches while preserving the existing
  `raw` client style as the default.
- `api-toolkit new service --profile saas-api-full` now checks in typed Go
  client output and its generated `client-check` target regenerates with
  `api-toolkit clients go --style typed`.
- `api-toolkit contracts lint`, `contracts diff`, and `clients go --style typed`
  now normalize OpenAPI 3.1 schema `type` arrays containing `null` and
  schema-level `examples` before parser validation. Contract linting also
  rejects Go client method, schema type, and parameter identifier collisions
  that would make typed client output unstable or unbuildable.
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
- `make contrib-release-notes-check` now reviews
  `github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit` behavior files in
  addition to supported adapters, integrations, middleware, bootstrap, and
  telemetry, so scaffold and contract-tooling behavior changes require
  release-note coverage.
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap` now exposes
  `APIService` and `APIServiceConfig` as a supported composition root for
  generated services, with safe admin-wrapper system endpoint mounting and
  startup checks.
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap.APIServiceConfig` now
  accepts `AdminAddr` and `AdminRouter` for a separate admin listener, and
  `APIService.AdminHandler()` exposes the composed admin handler for tests and
  custom server wiring.
- `github.com/aatuh/api-toolkit/contrib/v3/cache` and
  `github.com/aatuh/api-toolkit/contrib/v3/adapters/cacheredis` add
  supported contrib cache contracts and a Redis-backed cache adapter with TTL,
  delete, health-check, and reusable adapter-contract coverage.
- `github.com/aatuh/api-toolkit/contrib/v3/audit` and
  `github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres` add
  supported contrib audit-event contracts, reusable recorder-contract tests, and a
  transaction-aware Postgres audit recorder that stores actor type, tenant,
  action, resource, result, request ID, and redaction-checked metadata. The
  generated `saas-api-full` audit migration now includes `actor_type`.
- `github.com/aatuh/api-toolkit/v3/operations` adds additive write-side
  repository contracts plus lifecycle helpers for validating operation states,
  terminal states, and pending/running/succeeded/failed/canceled transitions.
- `github.com/aatuh/api-toolkit/contrib/v3/async` adds a supported contrib durable
  async worker runner with lease/complete/fail store contracts, bounded
  concurrency, graceful shutdown, low-cardinality metric hooks, and logs that
  avoid job payloads and raw handler errors.
- `github.com/aatuh/api-toolkit/contrib/v3/async` now includes an
  fail-closed handler mux for routing leased jobs by sanitized
  low-cardinality kind, allowing one durable queue or outbox to back multiple
  worker concerns without inspecting job payloads.
- `github.com/aatuh/api-toolkit/contrib/v3/adapters/operationpostgres` adds an
  supported Postgres-backed operation repository for pollable async
  operations, including tenant-scoped context helpers, JSON result/problem
  storage, create/update support, and fail-closed tenant validation.
- `github.com/aatuh/api-toolkit/contrib/v3/adapters/outboxpostgres` adds an
  supported Postgres transactional outbox adapter with enqueue, due-event
  leasing using `FOR UPDATE SKIP LOCKED`, lease-owner completion, retry
  backoff, dead-letter transition, and `contrib/async.Store` compatibility.
- `github.com/aatuh/api-toolkit/contrib/v3/objectstore` and
  `github.com/aatuh/api-toolkit/contrib/v3/adapters/objectstores3` add
  supported contrib object storage contracts, reusable contract-test helpers, and a
  raw HTTP S3-compatible adapter with SigV4 request signing, presigned URL
  hooks, content-type and object-size policy checks, metadata secret-shape
  rejection, not-found mapping, and a bucket health checker.
- `github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery` adds
  supported contrib outbound webhook delivery contracts with a fail-closed event
  catalog, tenant-scoped endpoint matching, HMAC-signed HTTP delivery, bounded
  retry backoff helpers, replay commands, sanitized attempt results, and
  `contrib/async` worker integration that keeps endpoint signing secrets out
  of durable job payloads.
- `github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres`
  adds a supported Postgres adapter for outbound webhook endpoint lookup,
  delivery enqueue, outbox job creation, attempt recording, and operator
  replay. Endpoint signing secrets are loaded through an application-owned
  `SecretResolver` instead of raw secret storage in the webhook endpoint table;
  generated `saas-api-full` migrations now include `event_id` and
  `last_status_code` on webhook delivery rows. The adapter also accepts the
  shared `webhookdelivery.EndpointPolicy` so generated development and
  integration services can allow localhost HTTP webhook targets without
  weakening production HTTPS defaults.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` and
  `github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog` now expose
  outbound webhook delivery observation hooks with bounded event type, outcome,
  and status-class labels that omit tenants, endpoint IDs, delivery IDs, URLs,
  payloads, secrets, and raw error strings.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/oidc` and
  `github.com/aatuh/api-toolkit/contrib/v3/integrations/auth/oidc` add
  supported provider-neutral OIDC/JWKS bearer-token middleware with optional
  discovery, issuer/audience and algorithm validation, tenant and scope claim
  mapping, JWKS health checks, env loading, and generated `saas-api-full`
  `--auth oidc` wiring.
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap.APIServiceConfig` now
  accepts named shutdown hooks so composed services can close auth, telemetry,
  or adapter background resources after the HTTP server stops.
- `middleware/auth/tenant.Options.RequireAllSources` now lets services require
  every configured tenant source to be present and equal before a handler runs,
  which supports authenticated-tenant-to-header mismatch checks.
- `github.com/aatuh/api-toolkit/contrib/v3/cmd/api-toolkit` adds the
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
- Generated `saas-api` Makefiles now make `coverage-check` enforce
  `COVERAGE_MIN` instead of only writing a coverage profile, so generated CI
  fails closed when test coverage drops below the configured floor.
- Generated `saas-api` Makefiles now install `govulncheck` under `.tools/bin`
  by default and invoke it through the overridable `GOVULNCHECK` variable, so
  scaffold checks do not require globally mutating the developer Go bin.
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
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now records
  bounded `idempotency_outcomes_total` Prometheus counters through
  `IdempotencyOutcomeHook`, and
  `github.com/aatuh/api-toolkit/contrib/v3/middleware/requestlog` now provides
  `IdempotencyOutcomeLogHook` with the same low-cardinality outcome shape.
  Generated `saas-api` services wire both hooks by default.
- `middleware/idempotency` now supports `Options.StorageKeyFunc` plus
  `TenantScopedStorageKeyFunc()` so multi-tenant services can hash
  client-supplied idempotency keys with tenant and actor scope before shared
  storage access. Generated `saas-api` services opt into the helper while
  preserving the original `Idempotency-Key` response header on replay.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now records
  bounded `health_status_changes_total` Prometheus counters through
  `HealthStatusChangeHook`, using only `from` and `to` health-status labels for
  scheduler transitions.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now treats
  `net/http.ServeMux` request patterns as route labels when chi route context is
  unavailable, preserving low-cardinality HTTP metrics for stdlib routers.
- Routes registered through `routecontracts` now attach bounded
  `routepolicy` observability labels. Contrib metrics records them through
  `http_route_policy_requests_total`, and contrib request logging emits
  `policy_*` fields without raw scopes, tenant sources, rate-limit policy
  names, or admin policy names.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now exposes
  `RoutePolicyLabels` so custom recorders can reuse the same bounded
  route-policy label normalization as the Prometheus recorder.
- `middleware/idempotency.Options` now includes additive `RequireKey` support.
  Generated SaaS services enable it so unsafe writes without `Idempotency-Key`
  fail closed with Problem Details 400 instead of executing untracked.
- Generated service READMEs and security guidance now document that unsafe
  writes without `Idempotency-Key` fail with Problem Details 400 and that
  generated idempotency storage keys are tenant and actor scoped.
- Generated services now load bootstrap router env controls for
  `TRUSTED_PROXIES` and rate-limit skip headers, failing startup on malformed
  proxy CIDRs or unsafe bypass settings.
- Generated services now register a shutdown hook for Redis idempotency clients
  so production stores are closed through the `bootstrap.APIService` lifecycle.
- Generated services now support `RATE_LIMIT_STORE=redis` and default to Redis
  rate limiting in production, including startup validation and Redis client
  shutdown hooks.
- Generated scaffold documentation now describes the local memory and
  production Redis rate-limit store defaults.
- Generated services now initialize contrib OpenTelemetry tracing from
  `OTEL_*` environment variables, fail startup when tracing is enabled without
  an OTLP endpoint, and close the tracer provider during service shutdown.
- Generated compose files now include a Redis service, healthcheck, persistent
  volume, and container-safe Redis address overrides for idempotency and rate
  limiting.
- `github.com/aatuh/api-toolkit/contrib/v3/telemetry.InitTracing` now returns
  an error when tracing is explicitly enabled without an OTLP endpoint instead
  of silently installing a noop exporter.
- `api-toolkit version` now prints Go runtime, main module, core module,
  contrib module, build commit, and build date metadata for release evidence.
- `api-toolkit version --json` now emits the same installed tool metadata in a
  stable machine-readable shape for release evidence and automation.
- Generated services now stamp `/version` from `appVersion`, `buildCommit`, and
  `buildDate`, and the generated Makefile/Dockerfile pass those fields through
  build flags with local `dev`/`unknown` defaults.
- `make v3-readiness-check` now runs focused compatibility-sensitive cleanup
  guardrails and is included in `make release-check` and release evidence logs,
  keeping major-version removal planning tied to roadmap, replacement guidance,
  and release-note requirements.
- CI governance now runs `make v3-readiness-check` explicitly alongside
  docs-check and contrib release-note/drift gates.
- `api-toolkit new service` now emits a pinned GitHub Actions workflow that runs
  `make finalize`, keeping generated services on the same test, build,
  OpenAPI golden, and contract lint path documented by the scaffold.
- The getting-started guide is now scaffold-first and verifies the generated
  service, OpenAPI golden, and contract lint/diff workflow instead of teaching a
  hand-written minimal starter as the primary path.
- Generated service Makefiles now include `fast-check`, `audit-check`,
  `coverage-check`, `test-race`, `vuln`, and `clean` targets so scaffold CI runs
  race tests and govulncheck in addition to build and contract checks.
- Generated service Makefiles now include optional `sbom-local` output through
  Syft, writing SPDX JSON to `.ci-result/sbom/sbom.spdx.json` without adding
  Syft to the default finalize path.
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap.APIServiceConfig` now
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
- `api-toolkit contracts lint`, `api-toolkit contracts diff`, and
  `contracttest.OpenAPICompatibilityFindings` now honor top-level OpenAPI
  `security` as inherited operation security and report
  `global_security_changed` when release-review specs change that default.
- `api-toolkit contracts lint` now emits a stable `GLOBAL`
  `security_scheme_undefined` finding when top-level OpenAPI security references
  a scheme missing from `components.securitySchemes`.
- `specs.Registry` now exposes `SetSecurity` for code-first top-level OpenAPI
  `security`, and `contracttest.SecuritySchemeDefinitionFindings` verifies
  those global requirements against `components.securitySchemes`.
- Generated `api-toolkit new service` scaffolds now use `specs.Registry`
  top-level OpenAPI security defaults in runtime docs and golden files while
  keeping protected write operation scopes explicit.
- Generated service READMEs now list admin-protected detailed health and pprof
  routes, and scaffold tests assert detailed health, metrics, and pprof all
  require `X-Admin-Key`.
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
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap` now exposes middleware
  stage identifiers, strict/dev middleware order helpers, and startup
  validation for custom APIService middleware order declarations.
- `github.com/aatuh/api-toolkit/contrib/v3/bootstrap` now exposes
  `StrictSaaSAPIMiddlewareOrder` for services that require the full production
  policy sequence of auth, tenant, and idempotency after the transport
  middleware stack, and generated `saas-api` services declare that order during
  startup validation.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now
  canonicalizes Prometheus HTTP metric labels so methods stay within standard
  HTTP verbs plus `OTHER`/`UNKNOWN`, invalid statuses collapse to `0`, and route
  labels are trimmed before series creation.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi` response
  validation now accepts `ResponseValidationOptions.ShouldValidate` so services
  can skip response buffering for streaming, upgrade, or large-download routes
  while keeping request validation enabled.
- `middleware/timeout.NewHard` now accepts `Options.EventHooks`, emitting
  bounded operator metadata for timeout, panic, and response-capture overflow
  outcomes without exposing panic values, paths, query strings, headers, or
  bodies.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now exposes
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
- `github.com/aatuh/api-toolkit/contrib/v3/adapters/ratelimittest` adds reusable
  rate limiter adapter contract coverage, and `ratelimitredis` now runs it to
  prove empty-key bypass, per-key isolation, retry-after, and refill behavior.
- `github.com/aatuh/api-toolkit/contrib/v3/adapters/healthchecktest` adds
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

- `github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/devheaders` now
  requires explicit dangerous-bypass opt-in and trusted-proxy configuration
  when enabled, while keeping exported config and middleware values comparable
  for v2 source compatibility.
- `github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics` now keeps the
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
- For `github.com/aatuh/api-toolkit/contrib/v3/middleware/auth/devheaders`,
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

- Billing contracts in `ports/billing.go` are now formally deprecated for new code. The same Stripe-shaped v2 model is available through the new compatibility package `github.com/aatuh/api-toolkit/v3/compat/billing`.
- `contrib/adapters/pgxpool.Adapter.StatSnapshot()` now copies plain-value pool stats directly from pgxpool instead of routing through the legacy `DatabaseStats` wrapper path.

### Upgrade notes

- Existing code that imports billing contracts from `ports` keeps working for the rest of v2, but new code should migrate to `github.com/aatuh/api-toolkit/v3/compat/billing` so the provider-shaped dependency is explicit before v3 extraction.
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
