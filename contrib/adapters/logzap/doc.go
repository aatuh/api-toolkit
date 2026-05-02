// Package logzap adapts zap logging to the core logger port.
//
// Use NewProduction or NewDevelopment when application wiring needs a concrete
// ports.Logger. The adapter owns zap-specific dependencies in contrib so stable
// core packages do not import zap.
package logzap
