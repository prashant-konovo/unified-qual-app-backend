package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Survey routes
// ══════════════════════════════════════════════════════════════

func registerSurveyRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/surveys", hs.Survey.ListSurveys)
		r.Post("/survey", hs.Survey.CreateSurvey)
		r.Put("/survey/{id}", hs.Survey.UpdateSurvey)
		r.Delete("/survey/{id}", hs.Survey.DeleteSurvey)
		// Phase 6: Survey extended
		r.Get("/survey/{id}/detail", hs.LS.GetSurveyDetail)
		r.Get("/survey/{id}/validate", hs.LS.ValidateSurvey)
		r.Get("/survey/{id}/crowds", hs.LS.GetSurveyCrowds)
		r.Put("/survey/{id}/close", hs.LS.CloseSurvey)
		r.Put("/survey/{id}/favorite", hs.LS.ToggleSurveyFavorite)
	})

	// Survey Responses (any authenticated user)
	r.Get("/survey-responses/{userId}", hs.Survey.GetSurveyResponses)
	r.Post("/survey-responses", hs.Survey.SubmitSurveyResponse)
}
