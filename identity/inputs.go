package identity

// EnsureInput contains the identity attributes coming from auth tokens.
type EnsureInput struct {
	Provider     string
	Subject      string
	Email        string
	FirstName    string
	LastName     string
	Language     string
	DefaultRoles []string
}

// UpdateProfileInput lists supported profile changes initiated by the user.
type UpdateProfileInput struct {
	UserID            string
	PreferredLanguage *string
}

// UpdateRolesInput replaces the roles assigned to a user.
type UpdateRolesInput struct {
	UserID string
	Roles  []string
}
