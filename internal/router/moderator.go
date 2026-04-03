package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Moderator routes — CRUD, availability, interviews
// ══════════════════════════════════════════════════════════════

func registerModeratorRoutes(r chi.Router, hs *handler.Handlers) {
	// Moderator management (admin + manager)
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/moderators", hs.Handler.ListModerators)
		r.Post("/moderators", hs.Handler.CreateModerator)
		r.Get("/moderators/{id}", hs.Handler.GetModerator)
		r.Put("/moderators/{id}", hs.Handler.UpdateModerator)
		r.Delete("/moderators/{id}", hs.Handler.DeleteModerator)
		r.Post("/moderators/bulk-upload", hs.Handler.BulkUploadModerators)
	})

	// Moderator availability — admin, manager, or moderator (own)
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Get("/moderators/{id}/availability", hs.Handler.GetModeratorAvailability)
		r.Post("/moderators/{id}/availability", hs.Handler.PostModeratorAvailability)
		r.Delete("/moderators/{id}/availability/{availabilityId}", hs.Handler.DeleteModeratorAvailability)
		r.Get("/moderators/{moderatorId}/timeslots", hs.Handler.GetModeratorTimeslots)
		r.Get("/timeslots/{id}/moderators/options", hs.Handler.GetTimeslotModeratorOptions)
	})

	// Interviews (admin, manager, moderator)
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Post("/interviews/schedule", hs.Handler.ScheduleInterview)
		r.Post("/interviews/{id}/cancel", hs.Handler.CancelInterview)
		r.Post("/interviews/{id}/reschedule", hs.Handler.RescheduleInterview)
	})
}
