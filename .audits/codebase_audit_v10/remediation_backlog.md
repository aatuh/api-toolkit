# Remediation Backlog

## Epic 1: Boundary Contract Hardening
Status: in progress

- [x] Ticket 1: Honor request cancellation during outbound HTTP retry backoff.
  Scope: `contrib/adapters/httpclient`
  Why: current retry sleep can continue after caller cancellation.
  Quality gate: `go test ./contrib/adapters/httpclient`
  Commit: next commit will record this change

- [x] Ticket 2: Disable detailed health exposure by default unless explicitly enabled.
  Scope: `endpoints/health`, docs/tests that lock the contract
  Why: current defaults contradict the documented operator-only health posture.
  Quality gate: `go test ./endpoints/health`
  Commit: next commit will record this change

- [x] Ticket 3: Make OpenAPI response capture preserve the first final status code.
  Scope: `contrib/middleware/openapi`
  Why: response validation should match real `http.ResponseWriter` behavior.
  Quality gate: `go test ./contrib/middleware/openapi`
  Commit: next commit will record this change

## Execution notes
- Work one ticket at a time.
- After each completed ticket: run its quality gate, update this file, and create a commit.
- After the epic is complete: run broader repo checks and mark the epic done.
