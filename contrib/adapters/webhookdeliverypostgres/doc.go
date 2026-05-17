// Package webhookdeliverypostgres provides a supported Postgres-backed
// registry, delivery store, attempt recorder, and replay adapter for
// contrib/webhookdelivery. Endpoint URL validation uses the shared
// webhookdelivery.EndpointPolicy option so production defaults stay strict
// while explicit local-development policies can allow HTTP endpoints.
package webhookdeliverypostgres
