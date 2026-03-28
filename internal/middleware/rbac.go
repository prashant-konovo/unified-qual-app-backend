package middleware

import (
	"net/http"
)

// RequireRoles returns middleware that checks the authenticated user
// holds at least one of the specified unified roles (admin, manager, moderator, observer, external).
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			for _, role := range user.Roles {
				if _, ok := allowed[role]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
		})
	}
}
