package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Booking routes
// ══════════════════════════════════════════════════════════════

func registerBookingRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/bookings", hs.Handler.ListBookings)
		r.Post("/bookings", hs.Handler.CreateBooking)
		r.Get("/bookings/{userId}", hs.Handler.GetBookingsByUser)
		r.Put("/bookings/{id}", hs.Handler.UpdateBooking)
		r.Put("/bookings/{id}/reward", hs.Handler.UpdateBookingReward)
	})
}
