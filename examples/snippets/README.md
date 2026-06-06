# Snippets

These snippets are copy-pasteable source files mirrored into public docs.
`GOTOOLCHAIN=local make docs-check` verifies the README snippet text matches the
source file and that the source file compiles.

## Catalog

- `minimal-existing-service`: smallest body-limit wrapper for an existing
  `net/http` service.
- `nethttp-migration`: before/after migration from plain `net/http` to toolkit
  body limit, Problem Details, validation, health, and timeout helpers. Verify
  it with `go test ./examples/snippets/nethttp-migration`.
