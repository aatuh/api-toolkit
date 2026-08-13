# Generated CLI downstream fixture

`scripts/downstream_compat_check.sh` runs the released and candidate
`api-toolkit new service` commands in separate temporary consumer directories.
For each generated service it runs `go mod tidy` and `go test ./...` with
`GOWORK=off`. This fixture is deliberately command-driven: the generated module
is the downstream consumer being verified.
