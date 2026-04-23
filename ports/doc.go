// Package ports provides toolkit-wide boundary contracts.
//
// Optional HTTP handler behavior is exposed through exported, narrow capability
// interfaces such as DetailedHealthManager, CachedHealthManager, and
// DocsHTMLModeProvider so external implementations can discover the full public
// contract without depending on package-private handler details.
//
// Most contracts in this package are intended to stay adapter-neutral. The
// billing contracts in billing.go and the database stats contracts in
// database.go remain stable in v2 as compatibility-sensitive surfaces and are
// documented separately in docs/ports-surface.md.
package ports
