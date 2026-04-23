// Package ports provides toolkit-wide boundary contracts.
//
// Optional HTTP handler behavior is exposed through exported, narrow capability
// interfaces such as DetailedHealthManager, CachedHealthManager, and
// DocsHTMLModeProvider so external implementations can discover the full public
// contract without depending on package-private handler details.
//
// Most contracts in this package are intended to stay adapter-neutral. The
// database stats contracts in database.go remain stable in v2 as a
// compatibility-sensitive surface. The provider-shaped billing contracts in
// billing.go also remain stable in v2, but they are deprecated in favor of
// github.com/aatuh/api-toolkit/v2/compat/billing. Both migration paths are
// documented separately in docs/ports-surface.md.
package ports
