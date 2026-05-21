// Package openapi provides supported-adapter OpenAPI validation middleware.
//
// Use New with WithSpec or LoadSpecFile when HTTP services need request
// validation against an OpenAPI document. ResponseValidationOptions can enable
// response checks, streaming opt-outs, body limits, and safe error hooks.
//
// Validation failures are mapped to Problem Details without exposing raw request
// bodies. The middleware does not own route registration or contract evolution;
// pair it with generated OpenAPI checks and release contract diffs.
package openapi
