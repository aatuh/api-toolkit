package domain

import "time"

type APIKey struct {
	ID             string
	OrganizationID string
	Name           string
	Prefix         string
	Scopes         []string
	ExpiresAt      *time.Time
	LastUsedAt     *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
}

func (k APIKey) Public() map[string]any {
	return map[string]any{
		"id":              k.ID,
		"organization_id": k.OrganizationID,
		"name":            k.Name,
		"prefix":          k.Prefix,
		"scopes":          append([]string(nil), k.Scopes...),
		"expires_at":      k.ExpiresAt,
		"last_used_at":    k.LastUsedAt,
		"revoked_at":      k.RevokedAt,
		"created_at":      k.CreatedAt,
	}
}
