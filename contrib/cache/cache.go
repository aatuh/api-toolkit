package cache

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrInvalidKey is returned when a cache key is empty after trimming space.
var ErrInvalidKey = errors.New("cache key is required")

// Store is the minimal cache contract used by contrib adapters and generated services.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// ValidateKey validates an application-owned cache key.
func ValidateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrInvalidKey
	}
	return nil
}

// CloneBytes returns a defensive copy of value.
func CloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte(nil), value...)
}
