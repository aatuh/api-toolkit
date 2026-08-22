package binding

// PublicError marks a validation error whose message is explicitly safe to
// return to an API client. Errors that do not implement this interface are
// retained for application logging but receive a generic public detail.
type PublicError interface {
	error
	// PublicMessage returns the error detail that is safe to include in a
	// validation Problem Details response.
	PublicMessage() string
}
