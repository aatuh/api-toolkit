package jwt

import "context"

type ctxKey string

const subjectCtxKey ctxKey = "jwt_subject"

// Subject contains identity information extracted from a JWT.
type Subject struct {
	UserID   string         `json:"user_id,omitempty"`
	Email    string         `json:"email,omitempty"`
	First    string         `json:"first,omitempty"`
	Last     string         `json:"last,omitempty"`
	Language string         `json:"language,omitempty"`
	Claims   map[string]any `json:"claims,omitempty"`
}

// WithSubject stores an authenticated subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	return context.WithValue(ctx, subjectCtxKey, subj)
}

// SubjectFromContext returns the subject if present.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	v := ctx.Value(subjectCtxKey)
	subj, ok := v.(Subject)
	if !ok {
		return Subject{}, false
	}
	if subj.UserID == "" && len(subj.Claims) == 0 {
		return Subject{}, false
	}
	return subj, true
}
