// Package objectstores3 provides a supported-adapter S3-compatible object store.
//
// Use New with Options to configure endpoint, region, bucket, credentials,
// content policy, and HTTP client behavior for objectstore.Store. Store
// implements put, get, delete, signed URL generation, not-found mapping,
// metadata mapping, and SigV4 signing for S3-compatible services.
//
// Validate object references and size/content-type policy before network I/O.
// Do not log access keys, signed URLs, object contents, or tenant-derived object
// keys as operational evidence.
package objectstores3
