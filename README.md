# api-toolkit

### Overview

Reusable building blocks for Go HTTP APIs that enforce Dependency
Inversion. Your application depends on stable `ports` interfaces, while
this toolkit ships adapters for popular libraries. Result: fewer direct
third‑party imports in your app, easier testing, and cleaner wiring.

Agent‑friendly by design: clear interfaces, small packages, sensible
defaults, and predictable wiring.

### Design goals

- Interfaces first: depend on `github.com/aatuh/api-toolkit/ports`.
- Adapters for third‑party libs; return interfaces, not concrete types.
- Consistent errors via RFC‑7807 (Problem+JSON).
- Small, composable middlewares with simple constructor options.

### Features

- Ports (interfaces)
  - Logger, Clock, IDGen
  - HTTPRouter, HTTPMiddleware
  - CORS handler, Security headers
  - Database: Pool, Tx, Rows, Row, Result, Stats
  - Health: Manager, Checkers, Results, Summaries
  - Docs: Manager, Info, Version, OpenAPI
  - Validator
  - MetricsRecorder (pluggable, with No‑op default)

- Environment & Config
  - `envvar`: adapter over `github.com/aatuh/envvar`
  - `config`: helpers to load env with "must" semantics

- Logging
  - `logzap`: Zap-based production logger returning `ports.Logger`

- HTTP Router & Middleware
  - `chi`: router and helpers as `ports.HTTPRouter` / `ports.HTTPMiddleware`
  - `middleware/cors`: CORS adapter (configurable defaults)
  - `middleware/secure`: security headers
  - `middleware/json`: JSON content-type enforcement and strict decoder
  - `middleware/timeout`: per-request timeouts
  - `middleware/maxbody`: request body size limits
  - `middleware/requestlog`: structured request logs
  - `middleware/ratelimit`: in-memory token bucket
  - `middleware/metrics`: request counters and durations via MetricsRecorder
  - `middleware/trace`: W3C Trace Context (traceparent) with safe defaults

- HTTP Helpers
  - `httpx`: RFC‑7807 Problem+JSON helper
  - `httpx/recover`: panic recovery that emits Problem+JSON
  - `response_writer`: success JSON encoder

- Health
  - `health`: manager + built‑in checkers (basic, DB, memory)
  - `health/handlers`: liveness, readiness, and detailed endpoints

- Docs
  - `docs`: serves HTML, version, and OpenAPI JSON
  - `docs/handlers`: routes for docs endpoints

- Database
  - `pgxpool`: adapter for `github.com/jackc/pgx/v5/pgxpool`
  - `txpostgres`: transactional helper over the pool
  - `migrator`: migration engine supporting embed.FS
  - `adapters/migrate`: CLI-friendly migrator wiring

- IDs & Time
  - `idgen`: ULID generator returning `ports.IDGen`
  - `clock`: system clock returning `ports.Clock`

- Validation
  - `validation`: adapter for `github.com/go-playground/validator/v10`

### Package map

- `ports` — core interfaces for all boundaries
- `envvar`, `config` — environment/config loading
- `logzap` — logger adapter (returns `ports.Logger`)
- `chi` — HTTP router adapter (returns `ports.HTTPRouter`)
- `middleware/*` — cors, secure, json, timeout, maxbody, requestlog,
  ratelimit, metrics, trace
- `httpx`, `httpx/recover` — error helpers and panic recovery
- `response_writer` — success JSON writer
- `health`, `health/handlers` — health manager and routes
- `docs`, `docs/handlers` — docs manager and routes
- `pgxpool`, `txpostgres` — database adapters
- `migrator`, `adapters/migrate` — migrations
- `idgen`, `clock` — utilities behind interfaces
- `validation` — input validation adapter

### Quickstart (wiring in main)

```go
// Logger and config
log := logzap.NewProduction()            // ports.Logger
cfg := config.MustLoadFromEnv()          // uses envvar under the hood

// Router and core middleware
r := chi.New()                           // ports.HTTPRouter
mw := chi.NewMiddleware()                // ports.HTTPMiddleware
r.Use(mw.RequestID())
r.Use(mw.RealIP())
r.Use(recoverx.Middleware(log))          // Problem+JSON on panic

// Standard middlewares
cors := corsmw.New()
r.Use(cors.Handler(corsmw.DefaultOptions()))
r.Use(securemw.New().Middleware())
r.Use(jsonmw.New(true).Handler)
r.Use(timeoutmw.New(5*time.Second).Handler)
r.Use(maxbody.New(1<<20).Handler)
r.Use(requestlog.New(log).Handler)
r.Use(metricsmw.New(nil).Handler)        // nil → Noop metrics
r.Use(tracemw.Middleware(tracemw.Options{TrustIncoming: false}))

// Health and docs
hm := health.New()
health.NewHandler(hm).RegisterRoutes(r)
docs.NewHandler(docs.New()).RegisterRoutes(r)
```

### Problem+JSON and success responses

```go
// Error
httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Detail: "invalid"})

// Success
response_writer.WriteJSON(w, http.StatusOK, payload)
```

### Validation

```go
v := validation.New()
if err := v.ValidateStruct(ctx, &dto); err != nil {
  httpx.WriteProblem(w, 400, httpx.Problem{Detail: err.Error()})
  return
}
```

### Migrations with embed

```go
//go:embed migrations/*.sql
var fsys embed.FS

m, _ := migrator.New(migrator.Options{FS: fsys, Logger: log})
_ = m.Up(".")
```

### Database adapters

```go
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil { /* handle */ }
tx := txpostgres.New(pool)
```

### Metrics integration

Provide an implementation of `ports.MetricsRecorder` to record counts
and durations. Passing `nil` to `metricsmw.New(nil)` uses a No‑op
implementation.

### Conventions for applications

- Import toolkit interfaces/adapters, not third‑party libs, in app code.
- Handlers: decode → validate → call service → encode.
- Use `httpx` for errors and `response_writer` for successes.

### Version and requirements

- Go 1.25+
- Adapters pin widely used libs (chi, zap, pgx, validator).

### Recommended Usage

- Wire dependencies via `ports` and adapters; avoid direct imports in
  the application layer.
- Prefer non‑interactive Make targets; verify with `make health`.
- Do not edit generated files; run `make codegen` when annotations
  change.
