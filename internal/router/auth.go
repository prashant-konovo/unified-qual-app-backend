package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Auth routes — JWT required, no role restriction
// ══════════════════════════════════════════════════════════════

func registerAuthRoutes(r chi.Router, hs *handler.Handlers) {
	r.Put("/auth/password", hs.Auth.AuthPassword)
	r.Get("/auth/me", hs.Auth.AuthMe)
}
