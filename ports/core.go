package ports

import (
	"context"
	"time"
)

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

// Validator defines the interface for input validation.
type Validator interface {
	Validate(ctx context.Context, value interface{}) error
	ValidateStruct(ctx context.Context, obj interface{}) error
	ValidateField(ctx context.Context, obj interface{}, field string) error
}

// TxManager runs a function within a transaction boundary.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Migrator defines an interface for database migrations.
type Migrator interface {
	Up(ctx context.Context, dir string) error
	Down(ctx context.Context, dir string) error
	Status(ctx context.Context, dir string) (string, error)
	Close() error
}
