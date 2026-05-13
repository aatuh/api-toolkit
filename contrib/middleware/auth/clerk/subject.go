package clerk

import (
	"context"

	"github.com/aatuh/api-toolkit/v2/authorization"
)

type ctxKey string

const subjectCtxKey ctxKey = "clerk_subject"

// Subject contains the identity extracted from Clerk tokens.
type Subject struct {
	UserID   string
	Email    string
	First    string
	Last     string
	Language string
	TenantID string
	Scope    string
}

// WithSubject stores an authenticated subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	ctx = context.WithValue(ctx, subjectCtxKey, subj)
	if subj.UserID != "" {
		ctx = authorization.WithActor(ctx, authorization.Actor{UserID: subj.UserID})
	}
	return ctx
}

// SubjectFromContext returns the subject if present.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	v := ctx.Value(subjectCtxKey)
	if subj, ok := v.(Subject); ok && subj.UserID != "" {
		return subj, true
	}
	return Subject{}, false
}
