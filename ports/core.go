package ports

import "time"

// Logger is a tiny façade to avoid vendor lock-in.
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

// Clock allows deterministic tests.
type Clock interface {
	Now() time.Time
}

// IDGen generates unique IDs.
type IDGen interface {
	New() string
}
