# OpenAPI Contract Workflow Guide

Audience: maintainers and application teams using api-toolkit route contracts,
OpenAPI generation, contract linting, generated clients, or runtime validation.

The workflow is contract-first enough to make drift visible without forcing a
design-first framework.

## Source Of Truth

| Source | Role |
| --- | --- |
| `routecontracts` | Registers operation ID, method, path, auth, tenant, idempotency, rate-limit, admin, deprecation, and streaming metadata. |
| `specs` | Builds schemas, examples, parameters, responses, and OpenAPI document structure. |
| OpenAPI golden file | Reviewable generated artifact used for drift detection. |
| Contract lint | Fails missing route metadata, unsafe-write metadata, Problem Details responses, protected operator paths, and schema issues. |
| Contract diff | Fails removed operations, operation ID changes, request tightening, response content removal, schema narrowing, and route-policy drift. |
| Generated docs and clients | Consumer-facing output that must be reviewed with the OpenAPI diff. |

## Route Metadata

Each non-trivial route should document:

- operation ID,
- method and path,
- request and response schemas,
- auth and scope requirements,
- tenant source requirements,
- idempotency requirement for unsafe writes,
- rate-limit metadata,
- admin/operator-only status,
- deprecation status,
- streaming or large-response caveat with `x-api-toolkit-streaming`.

## Local Workflow

1. Update route contracts and schemas.
2. Regenerate the OpenAPI golden file or generated docs in the application.
3. Run contract lint.
4. Run contract diff against the previous OpenAPI artifact.
5. Regenerate clients when the service owns generated clients.
6. Review release notes for behavior, compatibility, security, and generated
   client impact.

For CLI workflows:

```sh
api-toolkit contracts lint --openapi ./openapi.json
api-toolkit contracts diff --base ./openapi.previous.json --head ./openapi.json
api-toolkit clients go --openapi ./openapi.json --out ./client --package apiclient --style typed
```

## Runtime Validation

Request validation can run broadly when route contracts are complete. Response
validation buffers responses and should be used in tests, development, or
selected finite production routes.

Use `openapi.ResponseValidationOptions.ShouldValidate` to skip streaming,
websocket, SSE, large-download, and optional-writer routes. Keep request
validation enabled globally when possible.

## Drift Handling

Treat the OpenAPI diff as a release review artifact:

- additive operations are usually compatible,
- removed operations are breaking,
- changed operation IDs are breaking for generated clients,
- removed documented parameters or responses are breaking,
- added required parameters are breaking,
- schema type, required, property, or enum narrowing needs migration review,
- route-policy drift in auth, tenant, idempotency, rate-limit, admin, or
  deprecation metadata needs security review.

Document intentional drift in `docs/release-notes.md` and, when it affects
stable root packages, update `VERSIONING.md` or the relevant compatibility docs.
