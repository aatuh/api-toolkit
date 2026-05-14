package oidc

import (
	"context"

	"github.com/aatuh/api-toolkit/v2/authorization"
)

type ctxKey string

const subjectCtxKey ctxKey = "oidc_subject"

// Subject contains identity information extracted from a validated OIDC token.
type Subject struct {
	UserID   string
	Email    string
	TenantID string
	Scope    string
	Claims   map[string]any
}

// WithSubject stores an authenticated OIDC subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	ctx = context.WithValue(ctx, subjectCtxKey, subj)
	if subj.UserID != "" {
		ctx = authorization.WithActor(ctx, authorization.Actor{UserID: subj.UserID})
	}
	return ctx
}

// SubjectFromContext retrieves the OIDC subject from context.
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
