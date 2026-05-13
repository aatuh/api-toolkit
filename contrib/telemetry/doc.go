// Package telemetry provides contrib OpenTelemetry wiring helpers.
//
// The package keeps tracing and instrumented HTTP client dependencies outside
// the stable core module. Configure explicit timeouts and exporters in
// application bootstrap. InitTracing returns an error when tracing is enabled
// without an OTLP endpoint instead of silently disabling exporter setup.
package telemetry
