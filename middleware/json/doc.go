// Package jsonmw provides stable JSON request helpers and middleware.
//
// Use StrictDecoder for handlers that should reject malformed JSON and rely on a
// clear content-type contract. The middleware package is part of the stable core
// surface and should stay provider-neutral.
package jsonmw
