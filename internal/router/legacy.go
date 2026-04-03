package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Legacy routes — Phase 6/7 remaining endpoints
// ══════════════════════════════════════════════════════════════

func registerLegacyRoutes(r chi.Router, hs *handler.Handlers) {
	// ── Waiting Queue (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/waiting-queue", hs.Handler.GetWaitingQueue)
		r.Post("/waiting-queue", hs.Handler.AddToWaitingQueue)
		r.Delete("/waiting-queue/{id}", hs.Handler.RemoveFromWaitingQueue)
		r.Post("/match-slots", hs.Handler.TriggerMatching)
	})

	// ── Transcription — CastingWords integration (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/transcription/order", hs.Handler.CreateTranscriptionOrder)
		r.Get("/transcription/status/{orderId}", hs.Handler.GetTranscriptionStatus)
		r.Get("/transcription/transcript/{orderId}", hs.Handler.GetTranscript)
	})

	// ── SMS — Bandwidth integration (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/sms/send", hs.Handler.SendSMS)
	})

	// ── Payments (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/payments/timeslot", hs.LS.CreatePaymentReal)
		r.Post("/payments/custom-honorarium", hs.LS.CreateCustomHonorariumReal)
		r.Get("/payments/status-list", hs.LS.GetPaymentStatusListReal)
	})

	// ── Translations (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/translations/locales", hs.LS.GetLocalesReal)
		r.Put("/projects/{projectId}/topics/translations", hs.Handler.UpdateTopicTranslations)
	})

	// ── Notifications (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/notifications/email-template", hs.Handler.GetEmailTemplate)
		r.Post("/notifications/reminder", hs.Handler.SendReminder)
	})

	// ── Markets (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/markets", hs.LS.ListMarkets)
		r.Get("/markets/npi", hs.LS.ListMarketsNPI)
		r.Get("/market/{id}/crowdable_attributes", hs.LS.GetCrowdableAttributes)
	})

	// ── Timeslot Sub-resources (admin + manager) ──
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

	// ── Moderator Availability by subscription (admin + manager + moderator) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Get("/moderator/{modId}/availability/{subId}", hs.LS.GetModeratorAvailabilityBySub)
		r.Post("/moderator/{modId}/availability/{subId}", hs.LS.PostModeratorAvailabilityBySub)
		r.Put("/moderator/availability/{maId}", hs.LS.UpdateModeratorAvailabilityExt)
		r.Delete("/moderator/availability/{maId}", hs.LS.DeleteModeratorAvailabilityExt)
	})

	// ── Self-Service (admin + manager + moderator) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Get("/selfservice/noshow", hs.LS.GetNoShowCheck)
		r.Put("/selfservice/project/{pid}/timeslot/{tid}", hs.LS.MarkNoShow)
	})

	// ── User (authenticated, no role restriction) ──
	r.Get("/user/get-email/{id}", hs.Handler.GetUserEmail)
	r.Get("/user/{id}", hs.Handler.GetUser)
	r.Put("/user/{id}", hs.Handler.UpdateUser)
	r.Post("/user/upsert-user-time-zone-selection", hs.Handler.UpsertUserTimeZone)
	r.Put("/user/password_matches", hs.Handler.CheckPasswordMatches)

	// ── Event Logs (any authenticated) ──
	r.Post("/EventLogs", hs.Handler.CreateEventLog)

	// ── Salesforce Projects (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/salesforceprojects", hs.Handler.ListSalesforceProjects)
	})

	// ── User Roles (admin) ──
	r.Group(func(r chi.Router) {
		r.Use(adminOnly)
		r.Post("/user/add_roles", hs.LS.AddUserRoles)
		r.Delete("/user/delete_roles", hs.LS.DeleteUserRoles)
	})

	// ── Password Reset (behind JWT but no role restriction) ──
	r.Post("/reset-user-password/send-user-password-email", hs.LS.SendPasswordResetEmail)
	r.Post("/reset-user-password/check-qsTool-i2", hs.LS.CheckUserIsQsToolAndI2)
	r.Patch("/reset-user-password/patch-user/{user_id}", hs.MRA.PatchUser)
	r.Patch("/reset-user-password/patch-user-from-profile/{user_id}", hs.MRA.PatchUserFromProfile)
	r.Put("/reset-user-password/check-if-password-matches/{user_id}", hs.MRA.CheckPasswordMatchesMRA)

	// ── Unsubscribe / Comm Preferences (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/unsubscribe/check-user-comm-preference/{userId}", hs.LS.CheckUserCommPreference)
		r.Post("/unsubscribe/unsubscribe-user/{userId}", hs.LS.UnsubscribeUser)
	})

	// ── Project extensions (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/project/{projectId}/moderators_reset", hs.LS.ResetProjectModerators)
		r.Post("/project/{projectId}/handle-export", hs.LS.HandleProjectExport)
		r.Get("/project/{projectId}/available_moderators_count", hs.LS.GetAvailableModeratorsCount)
		r.Get("/project/{projectId}/unavailable_moderators", hs.LS.GetUnavailableModerators)
		r.Get("/project_manager/client", hs.LS.ListProjectManagers)
	})

	// ── Conference Links (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/project/{projectId}/add-conference-link", hs.LS.AddConferenceLink)
		r.Put("/update-conference-link", hs.LS.UpdateConferenceLinkHandler)
		r.Get("/conference-link/timeslot/{timeslotId}", hs.LS.GetConferenceLinkByTimeSlot)
	})

	// ── Notifications send (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/notifications/send", hs.Handler.SendNotificationEmail)
	})

	// ── Third-Party / Eligibility (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/third-party-integrate", hs.LS.ThirdPartyIntegrate)
		r.Post("/qual/eligibility", hs.LS.CheckQualEligibility)
	})

	// ── Translation Delete (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Delete("/projects/{projectId}/topics/translations/{translationKey}", hs.LS.DeleteTopicTranslation)
		r.Delete("/projects/{projectId}/translations/{langCode}", hs.LS.DeleteTranslation)
	})

	// ── External Payments (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/payments/external", hs.LS.CreateExternalPayment)
		r.Get("/payments/honorarium-reasons", hs.LS.GetHonorariumReasons)
		r.Get("/payments/interview-statuses", hs.LS.GetInterviewPaymentStatusList)
	})

	// ── LS Inquiry (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Put("/subscription/{subscriptionId}/inquiry_preview", hs.LS.UpdateInquiryPreview)
		r.Post("/custom_crowd_inquiry", hs.LS.CreateCustomCrowdInquiry)
	})

	// ── Google Calendar / Import placeholders (admin + manager + moderator) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Post("/moderator/{moderatorId}/import/start", hs.LS.StartModeratorImport)
		r.Get("/moderator/{moderatorId}/import/availability", hs.LS.GetImportedAvailability)
		r.Get("/moderator/{moderatorId}/import/status", hs.LS.GetImportStatus)
		r.Delete("/moderator/{moderatorId}/import/unlink", hs.LS.UnlinkImportedModerator)
		r.Put("/google-sheet/first-date", hs.LS.UpdateGoogleSheetFirstDate)
	})
}
