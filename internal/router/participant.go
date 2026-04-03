package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Participant routes
// ══════════════════════════════════════════════════════════════

func registerParticipantRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/participants", hs.Handler.ListParticipants)
		r.Post("/participants", hs.Handler.CreateParticipant)
		r.Get("/participants/{id}", hs.Handler.GetParticipant)
	})
}
