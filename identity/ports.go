package identity

import (
	"context"
	"time"
)

// Repo defines persistence requirements for the identity service.
type Repo interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByIdentity(ctx context.Context, provider, subject string) (*User, error)
	Update(ctx context.Context, u *User) error
	ListRoles(ctx context.Context, userID string) ([]string, error)
	ReplaceRoles(ctx context.Context, userID string, roles []string, at time.Time) error
}
