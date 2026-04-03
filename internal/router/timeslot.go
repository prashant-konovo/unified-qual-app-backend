package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Timeslot routes — CRUD + interview slot generation
// ══════════════════════════════════════════════════════════════

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
}
