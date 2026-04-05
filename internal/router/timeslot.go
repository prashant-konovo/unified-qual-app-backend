package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Timeslot routes — CRUD + interview slot generation
// ══════════════════════════════════════════════════════════════

// registerPublicTimeslotRoutes registers participant-facing slot endpoints (no JWT).
func registerPublicTimeslotRoutes(r chi.Router, hs *handler.Handlers) {
	r.Get("/slots", hs.Interview.GetAvailableSlots)
}

func registerTimeslotRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/timeslots", hs.Interview.ListTimeslots)
		r.Post("/timeslots", hs.Interview.CreateTimeslot)
		r.Get("/timeslots/{id}", hs.Interview.GetTimeslot)
		r.Put("/timeslots/{id}", hs.Interview.UpdateTimeslot)
		r.Delete("/timeslots/{id}", hs.Interview.DeleteTimeslot)
	})

	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/slots/generate", hs.Interview.GenerateSlots)
		r.Get("/ai/suggested-slots", hs.Interview.GetAISuggestions)
	})

	// ── Timeslot sub-resources (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/time_slot/{tsId}/moderators", hs.LS.GetTimeslotModerators)
		r.Get("/time_slot/{tsId}/moderators/options", hs.LS.GetTimeslotModeratorOptionsExt)
		r.Post("/time_slot/{tsId}/moderator", hs.LS.AssignTimeslotModerator)
		r.Delete("/time_slot/{tsId}/moderator/{modId}", hs.LS.UnassignTimeslotModerator)
		r.Get("/time_slot/{tsId}/observers", hs.LS.GetTimeslotObservers)
		r.Put("/time_slot/{tsId}/observers", hs.LS.UpdateTimeslotObservers)
		r.Post("/project/{pid}/time_slot/{tsId}/{action}", hs.MRA.CancelRescheduleAction)
	})

	// ── Waiting queue (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/waiting-queue", hs.Handler.GetWaitingQueue)
		r.Post("/waiting-queue", hs.Handler.AddToWaitingQueue)
		r.Delete("/waiting-queue/{id}", hs.Handler.RemoveFromWaitingQueue)
		r.Post("/match-slots", hs.Handler.TriggerMatching)
	})
}
