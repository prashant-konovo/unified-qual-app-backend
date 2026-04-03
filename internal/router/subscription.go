package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Subscription routes — CRUD + sub-resources
// ══════════════════════════════════════════════════════════════

func registerSubscriptionRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/subscriptions", hs.LS.ListSubscriptions)
		r.Post("/subscription", hs.LS.CreateSubscription)
		r.Get("/subscription/{id}", hs.LS.GetSubscription)
		r.Put("/subscription/{id}", hs.LS.UpdateSubscription)
		r.Delete("/subscription/{id}", hs.LS.DeleteSubscription)
		// Phase 6: Subscription sub-resources
		r.Get("/subscription/{id}/interviews", hs.LS.GetSubscriptionInterviews)
		r.Get("/subscription/{id}/crowds", hs.LS.GetSubscriptionCrowds)
		r.Get("/subscription/{id}/question_types", hs.LS.GetSubscriptionQuestionTypes)
		r.Get("/subscription/{subId}/inquiries", hs.LS.GetSubscriptionInquiries)
		r.Get("/subscription/{subId}/project/{pid}/inquiry", hs.LS.GetSubscriptionProjectInquiry)
		r.Get("/subscription/{subId}/project_surveys", hs.LS.GetSubscriptionProjectSurveys)
	})
}
