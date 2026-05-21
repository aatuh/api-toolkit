// Package objectstore defines supported-adapter object storage contracts.
//
// Ref, Store, PutOptions, GetResult, and SignedURLOptions describe the reusable
// boundary implemented by adapters such as objectstores3 and memory stores in
// generated services. ValidateRef and policy helpers keep bucket/key and
// content constraints explicit before adapter I/O.
//
// Object keys, signed URLs, and metadata can contain sensitive application
// context. Keep them out of logs, metrics labels, and release evidence unless an
// access-controlled incident review explicitly requires them.
package objectstore
