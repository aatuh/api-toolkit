// Package clock adapts the system clock to the core clock port.
//
// This wrapper is intentionally thin: behavior delegates to the standard time
// package, while the package gives application wiring a concrete implementation
// for ports.Clock.
package clock
