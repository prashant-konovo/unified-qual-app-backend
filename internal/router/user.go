package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// User routes — CRUD, roles, password reset, comm preferences
// ══════════════════════════════════════════════════════════════

func registerUserRoutes(r chi.Router, hs *handler.Handlers) {
	// ── User management (any authenticated) ──
	r.Get("/user/get-email/{id}", hs.Handler.GetUserEmail)
	r.Get("/user/{id}", hs.Handler.GetUser)
	r.Put("/user/{id}", hs.Handler.UpdateUser)
	r.Post("/user/upsert-user-time-zone-selection", hs.Handler.UpsertUserTimeZone)
	r.Put("/user/password_matches", hs.Handler.CheckPasswordMatches)

	// ── Password reset (any authenticated) ──
	r.Post("/reset-user-password/send-user-password-email", hs.LS.SendPasswordResetEmail)
	r.Post("/reset-user-password/check-qsTool-i2", hs.LS.CheckUserIsQsToolAndI2)
	r.Patch("/reset-user-password/patch-user/{user_id}", hs.MRA.PatchUser)
	r.Patch("/reset-user-password/patch-user-from-profile/{user_id}", hs.MRA.PatchUserFromProfile)
	r.Put("/reset-user-password/check-if-password-matches/{user_id}", hs.MRA.CheckPasswordMatchesMRA)

	// ── User roles (admin only) ──
	r.Group(func(r chi.Router) {
		r.Use(adminOnly)
		r.Post("/user/add_roles", hs.LS.AddUserRoles)
		r.Delete("/user/delete_roles", hs.LS.DeleteUserRoles)
	})

	// ── Comm preferences / unsubscribe (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/unsubscribe/check-user-comm-preference/{userId}", hs.LS.CheckUserCommPreference)
		r.Post("/unsubscribe/unsubscribe-user/{userId}", hs.LS.UnsubscribeUser)
	})
}
