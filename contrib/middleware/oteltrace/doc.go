// Package oteltrace provides OpenTelemetry tracing middleware for contrib users.
//
// Use it when application wiring wants spans around HTTP requests without adding
// OpenTelemetry dependencies to the stable core module. Keep request attributes
// low-cardinality and avoid recording secrets.
package oteltrace
