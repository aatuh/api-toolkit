package identity

import "time"

// User stores the local identity profile metadata.
type User struct {
	ID                string
	Provider          string
	Subject           string
	Email             string
	FirstName         string
	LastName          string
	PreferredLanguage string
	Roles             []string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
