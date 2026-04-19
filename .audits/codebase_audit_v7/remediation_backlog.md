# Remediation Backlog

Status legend:

- [ ] not done
- [x] done

## Epic E1 - Restore response semantics [x]

Description: Fix the shared response primitives so recovery, logging, metrics, idempotency replay, and OpenAPI response validation all observe the same visible response contract.

### Ticket E1-T1 - Fix `response_writer.Writer` commitment tracking [x]

Description: Preserve first final status semantics, keep informational responses legal, and treat `Flush()` as a commitment point for final responses.

Acceptance:
- Add regression tests for repeated `WriteHeader`, `Flush()`, and recovery-adjacent commitment checks.
- Run targeted tests for `response_writer`, `httpx/recover`, `contrib/middleware/requestlog`, and `contrib/middleware/metrics`.
- Commit after the ticket is complete.

### Ticket E1-T2 - Fix `response_writer.Capture` replay semantics [x]

Description: Preserve first final status semantics in buffered captures and return immutable body snapshots.

Acceptance:
- Add regression tests for repeated `WriteHeader` and `Body()` immutability.
- Run targeted tests for `response_writer`, `middleware/idempotency`, and `contrib/middleware/openapi`.
- Commit after the ticket is complete.

## Epic E2 - Harden docs output [x]

Description: Make the generated docs surface safe and deterministic even when config metadata contains HTML- or JS-significant characters.

### Ticket E2-T1 - Escape generated docs HTML and inline JS [x]

Description: Replace `fmt.Sprintf` HTML generation with templated output that escapes config values in text, attribute, and script contexts.

Acceptance:
- Add regression tests covering hostile title/description/path values.
- Run targeted tests for `endpoints/docs`.
- Commit after the ticket is complete.

## Epic E3 - Make config loading defensive [ ]

Description: Remove avoidable panics from the exported env loader helper.

### Ticket E3-T1 - Make `contrib/config.Loader` zero-value safe [ ]

Description: Lazily initialize the env adapter so exported loader methods behave consistently for zero values.

Acceptance:
- Add tests for zero-value loader use across string, required, duration, and CSV reads.
- Run targeted tests for `contrib/config`.
- Commit after the ticket is complete.
