# Chi Existing Service

This example shows how to add api-toolkit guardrails to an existing chi service
without scaffold generation and without contrib adapters.

The chi router still owns routing and path parameters. api-toolkit supplies
request body limits, request timeouts, health handlers, Problem Details
responses, JSON binding, and validation response helpers.

Run it from this directory:

```sh
go test ./...
go run .
```

Routes:

- `GET /livez`, `GET /readyz`, `GET /healthz`, `GET /health`: toolkit public
  health probes registered on chi.
- `POST /widgets`: JSON binding, required-field validation, positive quantity
  validation, body limits, and Problem Details errors.
- `GET /widgets/{id}`: chi path parameter extraction with toolkit response
  helpers.
