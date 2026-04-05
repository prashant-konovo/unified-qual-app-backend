package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Payment routes — timeslot payments, honorarium, external
// ══════════════════════════════════════════════════════════════

func registerPaymentRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/payments/timeslot", hs.LS.CreatePaymentReal)
		r.Post("/payments/custom-honorarium", hs.LS.CreateCustomHonorariumReal)
		r.Get("/payments/status-list", hs.LS.GetPaymentStatusListReal)
		r.Post("/payments/external", hs.LS.CreateExternalPayment)
		r.Get("/payments/honorarium-reasons", hs.LS.GetHonorariumReasons)
		r.Get("/payments/interview-statuses", hs.LS.GetInterviewPaymentStatusList)
	})
}
