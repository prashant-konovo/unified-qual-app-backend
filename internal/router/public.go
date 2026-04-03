package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Public routes — no JWT required
// ══════════════════════════════════════════════════════════════

func registerPublicRoutes(r chi.Router, hs *handler.Handlers) {
	// Auth endpoints
	r.Post("/auth/login", hs.Auth.AuthLogin)
	r.Post("/auth/magic-link", hs.Auth.AuthMagicLink)
	r.Post("/auth/refresh", hs.Auth.AuthRefresh)
	r.Post("/auth/accept-terms", hs.Auth.AuthAcceptTerms)
	r.Post("/auth/logout", hs.Auth.AuthLogout)
	r.Get("/auth/sso/config", hs.Auth.AuthSSOConfig)
	r.Post("/auth/sso/callback", hs.Auth.AuthSSOCallback)

	// Participant-facing endpoints
	r.Get("/survey/{surveyId}/public", hs.Survey.GetPublicSurvey)
	r.Post("/survey/submit", hs.Survey.SubmitParticipantSurvey)
	r.Get("/survey/{userId}", hs.Survey.GetParticipantSurveyResponse)
	r.Get("/slots", hs.Interview.GetAvailableSlots)

	// Conference login
	r.Post("/conf/{confId}/login", hs.Handler.ConferenceLogin)
	r.Get("/conf/{confId}/participants", hs.Handler.GetConferenceParticipants)

	// Webhooks / Callbacks (service-to-service)
	// Recording upload callback from Conference Service (S3 trigger → Lambda → here)
	r.Post("/chime/recording/meeting/{meetingId}", hs.Handler.RecordingUploadCallback)

	// Interview media for conference participants
	r.Get("/interview_media/{confHash}/{mediaId}/pages/{page}/media.pdf", hs.Handler.GetMediaPageForConference)
}
