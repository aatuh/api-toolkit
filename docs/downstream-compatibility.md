# Downstream Compatibility Kit

Audience: API consumers who want upgrade evidence for a service that adopts
api-toolkit HTTP contracts.

`compatkit` is an experimental test-support package for downstream service
tests. It lets an external service run api-toolkit compatibility checks against
either an in-process `http.Handler` or an explicitly configured base URL.

Use it when a service wants reusable evidence for:

- readiness and version endpoint response stability,
- Problem Details response shape,
- OpenAPI compatibility against a checked-in previous document,
- custom HTTP checks that should keep passing after toolkit upgrades.

Do not use it as a substitute for package-local unit tests, authz tests, or
provider-specific integration tests. It is service-level compatibility evidence,
not a full production certification.

## Minimal Handler Test

```go
func TestServiceCompatibility(t *testing.T) {
	mux := service.NewHandler()

	compatkit.Run(t, compatkit.Suite{
		Target: compatkit.Target{Handler: mux},
		Checks: compatkit.StableHTTPChecks(compatkit.StableHTTPConfig{
			ReadinessPath: "/readyz",
			OpenAPIPath:   "/openapi.json",
			PreviousOpenAPI: []byte(previousOpenAPIJSON),
			ProblemRequest: compatkit.Request{
				Method: http.MethodPost,
				Path:   "/widgets",
				Body:   []byte(`{"name":""}`),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			},
			ProblemStatus: http.StatusBadRequest,
		}),
	})
}
```

## Base URL Test

Use `Target{BaseURL: "http://127.0.0.1:8080"}` only when the test already owns
the service lifecycle. `compatkit` never discovers services by itself and never
rewrites files. Handler targets are called in-process; base URL targets call
only the URL configured by the test.

```go
result := compatkit.RunChecks(context.Background(), compatkit.Suite{
	Target: compatkit.Target{BaseURL: serverURL},
	Checks: []compatkit.Check{{
		Name: "version",
		Request: compatkit.Request{Method: http.MethodGet, Path: "/version"},
		Expect: compatkit.ExpectAll(
			compatkit.ExpectStatus(http.StatusOK),
			compatkit.ExpectHeaderContains("Content-Type", "application/json"),
		),
	}},
})
if err := result.Error(); err != nil {
	t.Fatal(err)
}
```

## Safety Defaults

Each request gets a fresh context with a default 5 second timeout. Response
bodies are capped at 1 MiB unless `Suite.MaxBodyBytes` overrides the limit.
Check paths must be relative to the configured target, so a single check cannot
bypass the suite target with an absolute URL.

`compatkit` is classified as `[experimental]` and `direct-tests` in
`docs/package-classification.tsv`. Promoting it to stable requires the stable
API review board process in `docs/governance.md`.
