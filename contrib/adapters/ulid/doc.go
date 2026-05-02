// Package ulid adapts ULID generation to the core ID generator port.
//
// The wrapper keeps ULID-specific dependencies in contrib while giving
// application wiring a concrete monotonic identifier generator.
package ulid
