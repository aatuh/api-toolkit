# Input-Size Threat Review

Audience: maintainers, application teams, and release reviewers checking
request-size and capture-size boundaries before changing stable HTTP surfaces.

This review is the canonical cross-package map for input-size controls in the
stable core. It covers request headers, request bodies, JSON decoding, query
parameters, multipart uploads, idempotency replay capture, and hard-timeout
response capture. Keep this document synchronized with the source defaults
before widening a limit or adding a new body parser.

## Limit Matrix

| Surface | Limit owner | Default or required bound | Threat / failure mode | Application review rule |
| --- | --- | --- | --- | --- |
| `httpx.HeaderLimits` | `httpx.HeaderLimitsStrict`, `HeaderLimitsBalanced`, `HeaderLimitsRelaxed` | Strict: 32 KiB and 40 header values. Balanced: 64 KiB and 100 header values. Relaxed: 1 MiB and 200 header values. `ApplyServer` sets `http.Server.MaxHeaderBytes` when `MaxBytes > 0`; `Check` uses `HeaderBytes` and `HeaderCount` as an application-level approximation. | Header-count or header-byte amplification before handlers run. | Pick one preset at server construction and keep proxy/load-balancer header limits at least as strict. Treat `HeaderBytes` as approximate guardrail evidence, not as a replacement for server and edge limits. |
| `middleware/maxbody.Options.MaxBytes` | `middleware/maxbody.New` | Must be greater than zero. The middleware wraps `http.MaxBytesReader` when a request body exists. | Oversized finite request bodies consume parser memory, CPU, or downstream storage. | Apply before JSON, webhook, multipart, or custom body parsing. Route-level sizes should be justified by the route contract and tested with an oversized request. |
| `binding.JSONConfig.MaxBytes` | `binding.DecodeJSON` | When `MaxBytes > 0`, reads through `io.LimitReader(MaxBytes+1)` and returns a `too_large` field error when exceeded. Zero leaves body size uncapped in this helper. | JSON bodies can be valid but too large to buffer safely. | Set `JSONConfig.MaxBytes` for direct binding use, or pair binding with `middleware/maxbody` on the route. Unknown-field rejection and required-object checks are parser strictness, not size limits. |
| `middleware/json.StrictDecoder` | `middleware/json.Options.RequireJSON` and `StrictDecoder` | `RequireJSON` gates body-bearing `POST`, `PUT`, and `PATCH` requests by media type. `StrictDecoder` calls `DisallowUnknownFields`; neither path caps body bytes. | Content-type enforcement can be mistaken for resource bounding. | Use JSON middleware for media-type policy only. Pair it with `middleware/maxbody` or `binding.JSONConfig.MaxBytes` before reading the body. |
| `middleware/querylimits.Options` | `middleware/querylimits.New` | Empty options default to `MaxParams=100`, `MaxKeyLength=100`, `MaxValueLength=2048`, `LimitParam=limit`, and `MaxLimit=100`; negative limits fail construction. | Query-key, query-value, repeated-parameter, or pagination-limit amplification. | Apply before handlers that decode query state. Larger values need route-specific rationale and regression coverage for over-limit rejection. |
| `upload.Config` | `upload.DecodeMultipart` | Empty config defaults `MaxRequestBytes` to 32 MiB and `MaxMemory` to 32 MiB. Optional `MaxFileBytes`, `RequiredFiles`, and `AllowedContentTypes` add per-route validation. | Multipart parsing can buffer bounded request data and expose oversized or unexpected files. | Set `MaxRequestBytes` and `MaxFileBytes` per upload route. Treat `MaxMemory` as the multipart parser storage threshold, not malware scanning, persistence policy, or a per-file-count limit. |
| `middleware/idempotency.Options` | `middleware/idempotency.New`, `readBody`, and response replay capture | Empty options default `MaxBodyBytes` and `MaxResponseBytes` to 1 MiB each; negative values fail construction. Oversized request bodies return 413. Oversized captured responses mark the idempotency outcome ambiguous and return 503. | Replay storage can buffer raw request bodies and finite response bodies. Large responses can exhaust memory or durable idempotency storage. | Exclude streaming, upgrade, and large-download routes from idempotency replay capture. Size durable stores for configured request and response limits, and keep raw keys and bodies out of logs, metrics, and release evidence. |
| `middleware/timeout.Options.MaxCaptureBytes` | `middleware/timeout.NewHard` | Zero defaults to 1 MiB. Negative values fail construction. Capture overflow returns a Problem Details error instead of truncating a response. | Hard-timeout response buffering can consume memory and drops optional writer interfaces. | Use hard timeout only on finite responses. Do not wrap streaming, SSE, websocket, hijack, or large-download routes without an explicit route override. |

## Review Procedure

1. Map every public route to the input channels it accepts: headers, query
   parameters, JSON body, multipart body, raw body, or replay capture.
2. Apply a body-size limit before the first full-body read. Prefer
   `middleware/maxbody` for route middleware and `binding.JSONConfig.MaxBytes`
   for direct JSON binding helpers.
3. Keep query and pagination limits fail-closed before handler side effects.
4. For multipart routes, choose request, memory, and file-size limits together.
   File scanning, storage, count limits, and object-key policy remain
   application-owned.
5. For idempotent writes, confirm the route response is finite and within
   `MaxResponseBytes`. Streaming, upgrade, SSE, and large-download routes need
   an opt-out or a different replay design.
6. For hard timeouts, size `MaxCaptureBytes` for finite success and error
   responses only. Hard timeout does not make streaming or optional
   `http.ResponseWriter` interfaces safe.
7. Record any limit increase in the route contract, release notes, or service
   operations guide with the expected request/response size and the oversized
   rejection evidence.

## Source Anchors

- `httpx/header_limits.go`
- `middleware/maxbody/maxbody.go`
- `binding/binding.go`
- `middleware/json/json.go`
- `middleware/querylimits/querylimits.go`
- `upload/upload.go`
- `middleware/idempotency/idempotency.go`
- `middleware/idempotency/capture.go`
- `middleware/timeout/timeout.go`
