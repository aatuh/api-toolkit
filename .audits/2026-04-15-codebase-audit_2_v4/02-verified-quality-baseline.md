# Verified Quality Baseline

Date: 2026-04-16

Code baseline: `a95fa32` (`build: pin local quality toolchain`)

Repository quality gate status: pass

Verified command:

```bash
GOTOOLCHAIN=local make finalize
```

Observed local environment:

- `go version go1.26.1-X:nodwarf5 linux/amd64`
- `golangci-lint v2.11.4`
- `gosec v2.25.0`
- `govulncheck v1.2.0`
- `apidiff v0.0.0-20260410095643-746e56fc9e2f`

Notes:

- The repository compatibility target remains Go `1.24.x` in `README.md` and CI.
- The local QA tool versions above are pinned in `Makefile` to keep audit and local gate results reproducible.
- `make finalize` completed `tools`, `fmt`, `lint`, `vuln`, `gosec`, `api-check`, `tidy`, `test`, `test-race`, `fuzz`, and `clean` successfully.
