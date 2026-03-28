package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const userClaimsKey contextKey = "userClaims"

// UserClaims holds the authenticated user's identity extracted from a Cognito JWT.
type UserClaims struct {
	Sub      string   // Cognito user sub (unique ID)
	Email    string   // email claim
	Username string   // cognito:username
	Groups   []string // cognito:groups — raw Cognito groups
	Roles    []string // Unified roles derived from Cognito groups (admin, manager, moderator, observer, external)
}

// SetUser stores claims in the request context.
func SetUser(r *http.Request, claims *UserClaims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userClaimsKey, claims))
}

// GetUser retrieves claims from the request context. Returns nil if unauthenticated.
func GetUser(r *http.Request) *UserClaims {
	v, _ := r.Context().Value(userClaimsKey).(*UserClaims)
	return v
}
