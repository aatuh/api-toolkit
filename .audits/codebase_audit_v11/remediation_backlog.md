# Remediation Backlog

## Epic 1: Runtime Contract Corrections
Status: done

- [x] Ticket 1: Restore embedded migration loading for `embed.FS` sources.
  Scope: `contrib/migrator`
  Why: current `EmbeddedFSs` handling skips the documented `migrations/` layout and silently loads no embedded migrations.
  Quality gate: `go test ./contrib/migrator`
  Commit: completed in the ticket commit

- [x] Ticket 2: Make custom health checkers fail closed instead of panicking on nil functions.
  Scope: `endpoints/health`
  Why: a wiring mistake in `NewCustomChecker*` currently turns health checks into a panic path.
  Quality gate: `go test ./endpoints/health`
  Commit: completed in the ticket commit

- [x] Ticket 3: Stop bootstrap profiles from validating query-limit config when query limits are disabled.
  Scope: `contrib/bootstrap`
  Why: disabled query guardrails should not block profile construction on stale or invalid query-limit options.
  Quality gate: `go test ./contrib/bootstrap`
  Commit: completed in the ticket commit

## Execution notes
- Work one ticket at a time.
- After each completed ticket: run its quality gate, update this file, and create a commit.
- After the epic is complete: run broader repo checks and mark the epic done.
- Epic close checks completed: `make finalize`
