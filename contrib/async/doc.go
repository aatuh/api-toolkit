// Package async provides experimental worker runner contracts for durable async jobs.
//
// The package includes a bounded runner, low-cardinality metric hooks, safe
// failure-message helpers, and a job-kind handler mux for sharing one durable
// queue across multiple worker concerns without routing on unbounded payload
// data.
package async
