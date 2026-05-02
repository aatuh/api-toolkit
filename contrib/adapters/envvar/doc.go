// Package envvar loads environment-backed configuration through explicit adapter helpers.
//
// Safe startup paths
// ------------------
//
// The adapter keeps legacy panic-oriented methods (Must*) and adds Try* variants
// that return parse errors instead of panicking:
//
//	token, err := adapter.TryGet("TOKEN")
//	if err != nil { ... }
//
//	// or
//	if err := adapter.TryLoadEnvFiles([]string{"./.env"}); err != nil { ... }
//
// This allows callers to keep startup failure strictness where desired while
// preserving a non-panicking path for validation and diagnostics.
//
// Bind helpers
// ------------
//
// Use TryBind/TryBindWithPrefix when loading config structs if you want to handle
// parse errors explicitly:
//
//	var cfg Config
//	if err := adapter.TryBindWithPrefix(&cfg, "APP_"); err != nil { ... }
package envvar
