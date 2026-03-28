package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func New(h *handler.Handler, jwtAuth *middleware.JWTAuth) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "IC-Auth", "CognitoToken"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Brand"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (public)
	r.Get("/health", h.Health)

	// API v1 routes — matches the frontend axios baseURL suffix /v1
	r.Route("/v1", func(r chi.Router) {
		// ── Public: Auth endpoints (no JWT required) ──
		r.Post("/auth/login", h.AuthLogin)
		r.Post("/auth/magic-link", h.AuthMagicLink)
		r.Post("/auth/refresh", h.AuthRefresh)
		r.Get("/auth/sso/config", h.AuthSSOConfig)
		r.Post("/auth/sso/callback", h.AuthSSOCallback)

		// ── Public: Participant-facing endpoints ──
		r.Get("/survey/{surveyId}/public", h.GetPublicSurvey)
		r.Post("/survey/submit", h.SubmitParticipantSurvey)
		r.Get("/survey/{userId}", h.GetParticipantSurveyResponse)
		r.Get("/slots", h.GetAvailableSlots)

		// ── Protected: All remaining routes require valid JWT ──
		r.Group(func(r chi.Router) {
			r.Use(jwtAuth.Middleware)

			// Auth (authenticated)
			r.Put("/auth/password", h.AuthPassword)
			r.Get("/auth/me", h.AuthMe)

			// ── Projects (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/projects", h.ListProjects)
				r.Post("/project", h.CreateProject)
				r.Get("/project/{id}", h.GetProject)
				r.Put("/project/{id}", h.UpdateProject)
				r.Delete("/project/{id}", h.DeleteProject)
			})

			// ── Surveys (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/surveys", h.ListSurveys)
				r.Post("/survey", h.CreateSurvey)
				r.Put("/survey/{id}", h.UpdateSurvey)
				r.Delete("/survey/{id}", h.DeleteSurvey)
			})

			// ── Survey Responses (any authenticated user) ──
			r.Get("/survey-responses/{userId}", h.GetSurveyResponses)
			r.Post("/survey-responses", h.SubmitSurveyResponse)

			// ── Timeslots (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/timeslots", h.ListTimeslots)
				r.Post("/timeslots", h.CreateTimeslot)
				r.Get("/timeslots/{id}", h.GetTimeslot)
				r.Put("/timeslots/{id}", h.UpdateTimeslot)
				r.Delete("/timeslots/{id}", h.DeleteTimeslot)
			})

			// ── Interview Slots (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/slots/generate", h.GenerateSlots)
				r.Get("/ai/suggested-slots", h.GetAISuggestions)
			})

			// ── Moderators (admin + manager for mgmt; moderator for own schedule) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/moderators", h.ListModerators)
				r.Post("/moderators", h.CreateModerator)
				r.Get("/moderators/{id}", h.GetModerator)
				r.Put("/moderators/{id}", h.UpdateModerator)
				r.Delete("/moderators/{id}", h.DeleteModerator)
				r.Post("/moderators/bulk-upload", h.BulkUploadModerators)
			})
			// Moderator availability — admin, manager, or moderator (own)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Get("/moderators/{id}/availability", h.GetModeratorAvailability)
				r.Post("/moderators/{id}/availability", h.PostModeratorAvailability)
				r.Get("/moderators/{moderatorId}/timeslots", h.GetModeratorTimeslots)
				r.Get("/timeslots/{id}/moderators/options", h.GetTimeslotModeratorOptions)
			})

			// ── Interviews (any authenticated user: admin, manager, moderator) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Post("/interviews/schedule", h.ScheduleInterview)
				r.Post("/interviews/{id}/cancel", h.CancelInterview)
				r.Post("/interviews/{id}/reschedule", h.RescheduleInterview)
			})

			// ── Participants (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/participants", h.ListParticipants)
				r.Post("/participants", h.CreateParticipant)
				r.Get("/participants/{id}", h.GetParticipant)
			})

			// ── Bookings (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/bookings", h.ListBookings)
				r.Post("/bookings", h.CreateBooking)
				r.Get("/bookings/{userId}", h.GetBookingsByUser)
				r.Put("/bookings/{id}", h.UpdateBooking)
				r.Put("/bookings/{id}/reward", h.UpdateBookingReward)
			})

			// ── Subscriptions (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/subscriptions", h.ListSubscriptions)
				r.Post("/subscription", h.CreateSubscription)
				r.Get("/subscription/{id}", h.GetSubscription)
				r.Put("/subscription/{id}", h.UpdateSubscription)
				r.Delete("/subscription/{id}", h.DeleteSubscription)
			})

			// ── Waiting Queue (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/waiting-queue", h.GetWaitingQueue)
				r.Post("/waiting-queue", h.AddToWaitingQueue)
				r.Delete("/waiting-queue/{id}", h.RemoveFromWaitingQueue)
				r.Post("/match-slots", h.TriggerMatching)
			})

			// ── Conference (admin + manager + moderator) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Post("/meetings/{meetingId}/action/{action}", h.MeetingAction)
				r.Post("/meeting/{meetingId}/universal", h.MeetingUniversalJoin)
			})

			// ── Payments (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/payments/timeslot", h.CreatePayment)
				r.Post("/payments/custom-honorarium", h.CreateCustomHonorarium)
				r.Get("/payments/status-list", h.GetPaymentStatusList)
			})

			// ── Translations (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/translations/locales", h.GetLocales)
				r.Put("/projects/{projectId}/topics/translations", h.UpdateTopicTranslations)
			})

			// ── Notifications (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/notifications/email-template", h.GetEmailTemplate)
				r.Post("/notifications/reminder", h.SendReminder)
			})

			// ── Admin (requires admin role only) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin"))
				r.Get("/admin/users", h.ListAdminUsers)
				r.Post("/admin/users", h.CreateAdminUser)
			})
		})
	})

	return r
}
