package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/InCrowd/unified-qual-api/internal/handler"
)

// ══════════════════════════════════════════════════════════════
// Admin routes — admin-only
// ══════════════════════════════════════════════════════════════

func registerAdminRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminOnly)
		r.Get("/admin/users", hs.Handler.ListAdminUsers)
		r.Post("/admin/users", hs.Handler.CreateAdminUser)
		r.Get("/qstoolAdmin/get-all-users", hs.MRA.ListAdminUsersMRA)
	})
}
