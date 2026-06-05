# Library-First Path

Audience: teams adding `api-toolkit` to an existing `net/http`, chi, or
app-owned router service.

Use this path when the service architecture already exists and you only need a
small set of API guardrails: bounded request bodies, JSON binding, Problem
Details responses, health/version endpoints, idempotency contracts, OpenAPI
metadata, or production-safe middleware defaults.

Do not start with the scaffold for this path. The scaffold is app-owned
generated code for new services. Existing services should import the smallest
package set that solves the immediate problem.

## Five-minute adoption

1. Add the root module:

```sh
go get github.com/aatuh/api-toolkit/v3
```

2. Pick one stable package from [core-package-guide.md](core-package-guide.md).

3. Wire it around your existing handlers:

```go
bodyLimit, err := maxbody.New(maxbody.Options{MaxBytes: 1 << 20})
if err != nil {
	return err
}

router.Handle("POST /widgets", bodyLimit.Handler(http.HandlerFunc(createWidget)))
```

4. Keep your router, persistence, auth provider, and business interfaces
app-owned unless a package guide says the toolkit owns that contract.

## Recommended first packages

| Need | Start with | Why |
| --- | --- | --- |
| JSON responses and Problem Details | `httpx`, `fielderrors` | Small response helpers with no router requirement. |
| Decode request bodies | `binding`, `middleware/maxbody` | Bounded JSON decoding and validation Problem Details. |
| Decode collections | `queryparams`, `endpoints/list` | Sorting, filtering, pagination, and cursor helpers. |
| Add request deadlines | `middleware/timeout` | Cooperative deadlines first; hard timeout only for finite responses. |
| Protect finite JSON routes | `middleware/json`, `middleware/querylimits`, `middleware/secure` | Content, query, and header guardrails. |
| Publish service status | `endpoints/health`, `endpoints/version` | Public probes and version metadata without adopting the scaffold. |
| Track route contracts | `routecontracts`, `routepolicy`, `specs` | OpenAPI and runtime policy metadata for app-owned routers. |

## Decision rules

- Import root packages directly; do not import contrib unless you need a
  supported adapter.
- Prefer package-local or app-owned interfaces over root `ports` for new
  business-specific boundaries.
- Apply hard-timeout buffering, idempotency response capture, and response
  validation only to finite JSON routes. Streaming, SSE, websocket, and large
  download routes need route-level opt-outs.
- Read [minimal-core.md](minimal-core.md) when the goal is the smallest
  dependency path.
- Read [contrib-adapters.md](contrib-adapters.md) only after choosing a core
  package that needs an adapter.

