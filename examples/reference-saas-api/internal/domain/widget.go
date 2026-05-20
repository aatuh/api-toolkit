package domain

import (
	"fmt"
	"time"
)

type Widget struct {
	ID        string
	TenantID  string
	Name      string
	Version   int64
	Deleted   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (w Widget) ETag() string {
	return fmt.Sprintf("%q", w.Version)
}

func (w Widget) Public() map[string]any {
	return map[string]any{
		"id":        w.ID,
		"tenant_id": w.TenantID,
		"name":      w.Name,
		"version":   w.Version,
	}
}
