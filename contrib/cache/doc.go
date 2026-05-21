// Package cache defines supported-adapter cache contracts for generated services.
//
// Store is the minimal get/set/delete interface implemented by adapters such as
// cacheredis. ValidateKey and CloneBytes provide shared behavior for key
// validation and defensive byte-slice handling, and cachetest contains the
// adapter contract suite.
//
// The package does not choose eviction, tenancy, serialization, or Redis key
// layout. Callers and adapters must keep cache keys service-scoped and avoid
// embedding secrets or raw personal data in keys.
package cache
