package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Notification routes — email, SMS, transcription, event logs
// ══════════════════════════════════════════════════════════════

func registerNotificationRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		// Email notifications
		r.Get("/notifications/email-template", hs.Handler.GetEmailTemplate)
		r.Post("/notifications/reminder", hs.Handler.SendReminder)
		r.Post("/notifications/send", hs.Handler.SendNotificationEmail)
		// SMS — Bandwidth integration
		r.Post("/sms/send", hs.Handler.SendSMS)
		// Transcription — CastingWords integration
		r.Post("/transcription/order", hs.Handler.CreateTranscriptionOrder)
		r.Get("/transcription/status/{orderId}", hs.Handler.GetTranscriptionStatus)
		r.Get("/transcription/transcript/{orderId}", hs.Handler.GetTranscript)
	})

	// Event logs (any authenticated)
	r.Post("/EventLogs", hs.Handler.CreateEventLog)
}
