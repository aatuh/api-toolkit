// Package uuid adapts UUID generation to the core ID generator port.
//
// The wrapper keeps UUID-specific dependencies in contrib while giving
// application wiring a concrete identifier generator for ports.IDGen.
package uuid
