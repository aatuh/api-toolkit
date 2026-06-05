# Package Classification

Audience: maintainers and adopters who need a rendered guide before consulting
the machine-readable package manifest.

The source of truth is `docs/package-classification.tsv`. This page exists so
the classification model is readable and searchable in rendered Markdown without
turning the TSV into prose-only data.

## API Statuses

| Status | Meaning |
| --- | --- |
| `stable` | Covered by the core v3 API compatibility promise and release API gate. |
| `compatibility-only` | Preserved for v3 compatibility, but not recommended as a new generic abstraction. |
| `supported-adapter` | Maintained contrib runtime adapter with direct tests, drift review, and behavior evidence; still outside stable core. |
| `experimental` | Maintained but not protected by stable or supported-adapter compatibility policy. |
| `wrapper-only` | Thin convenience wrapper where behavior belongs to the delegated adapter or dependency. |
| `test-only` | Test support package, not a public runtime API promise. |
| `example-only` | Runnable example package, build-smoke checked but not behavior-complete API. |
| `generated` | Generated example or scaffold code, validated by generation checks. |
| `tooling` | CLI or repository tooling package. |
| `excluded` | Internal or repository-only package outside public API classification. |

## Test Statuses

| Status | Meaning |
| --- | --- |
| `direct-tests` | Package has direct Go tests for owned behavior. |
| `wrapper-smoke-tested` | Tests prove interface satisfaction, constructor/defaults, disabled or nil behavior where applicable, and option propagation. |
| `test-support` | Package supports tests and may be covered by consuming contract tests. |
| `example-only` | Package is build-smoke checked; it is not behavior-complete coverage. |
| `generated` | Generated output is validated by generation or scaffold checks. |
| `tooling` | CLI/tooling behavior is validated by tooling tests. |
| `excluded` | Package is not part of public package coverage claims. |
| `needs-tests` | Release blocker until replaced with direct tests or a documented exception. |

## Root Adoption Summary

Recommended first-adoption packages are the small HTTP/API primitives described
in `docs/stable-core.md`. Packages classified as `compatibility-only` or called
out as scaffold-facing support should not be treated as recommended new generic
abstractions even when they remain protected for v3 compatibility.

For exact per-package status, use:

```sh
column -ts $'\t' docs/package-classification.tsv
```

Do not edit this page as a substitute for updating the TSV. Any package status
change must update `docs/package-classification.tsv`, `VERSIONING.md` when the
stable surface changes, and the relevant release notes or compatibility docs.
