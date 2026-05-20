package domain

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

func (r Role) Allows(required Role) bool {
	return roleRank(r) >= roleRank(required)
}

func roleRank(role Role) int {
	switch role {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

type Organization struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (o Organization) Public() map[string]any {
	return map[string]any{
		"id":         o.ID,
		"name":       o.Name,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
	}
}

type Membership struct {
	OrganizationID string
	UserID         string
	Role           Role
	CreatedAt      time.Time
}

func (m Membership) Public() map[string]any {
	return map[string]any{
		"organization_id": m.OrganizationID,
		"user_id":         m.UserID,
		"role":            string(m.Role),
		"created_at":      m.CreatedAt,
	}
}

type Invitation struct {
	ID             string
	OrganizationID string
	Email          string
	Role           Role
	TokenPrefix    string
	ExpiresAt      time.Time
	AcceptedAt     *time.Time
	CreatedAt      time.Time
}

func (i Invitation) Public() map[string]any {
	return map[string]any{
		"id":              i.ID,
		"organization_id": i.OrganizationID,
		"email":           i.Email,
		"role":            string(i.Role),
		"token_prefix":    i.TokenPrefix,
		"expires_at":      i.ExpiresAt,
		"accepted_at":     i.AcceptedAt,
		"created_at":      i.CreatedAt,
	}
}
