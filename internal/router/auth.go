package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Auth routes
// ══════════════════════════════════════════════════════════════

// registerPublicAuthRoutes registers auth endpoints that do NOT require JWT.
func registerPublicAuthRoutes(r chi.Router, hs *handler.Handlers) {
	r.Post("/auth/login", hs.Auth.AuthLogin)
	r.Post("/auth/magic-link", hs.Auth.AuthMagicLink)
	r.Post("/auth/refresh", hs.Auth.AuthRefresh)
	r.Post("/auth/accept-terms", hs.Auth.AuthAcceptTerms)
	r.Post("/auth/logout", hs.Auth.AuthLogout)
	r.Get("/auth/sso/config", hs.Auth.AuthSSOConfig)
	r.Post("/auth/sso/callback", hs.Auth.AuthSSOCallback)
}

// registerAuthRoutes registers auth endpoints that require JWT (no role restriction).
func registerAuthRoutes(r chi.Router, hs *handler.Handlers) {
	r.Put("/auth/password", hs.Auth.AuthPassword)
	r.Get("/auth/me", hs.Auth.AuthMe)
}
