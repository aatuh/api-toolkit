// Package audit defines supported-adapter audit event contracts for services.
//
// Event, Recorder, and ValidateEvent provide the reusable contract shared by
// generated services and audit adapters such as auditpostgres. The package owns
// field validation, result labels, actor/tenant/resource fields, and defensive
// metadata cloning.
//
// Audit metadata must stay redaction-safe before it reaches a Recorder. Do not
// put raw secrets, request bodies, provider payloads, or unbounded user input in
// metadata maps.
package audit
