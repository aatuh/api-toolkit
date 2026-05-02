// Package noop implements a no-op email sender.
//
// Use it in local development, tests, or disabled delivery paths where callers
// should exercise email wiring without sending messages. It intentionally has no
// external side effects.
package noop
