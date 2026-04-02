package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/logger"
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
	r.Use(logger.Middleware) // structured JSON logging with request ID
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
		r.Post("/auth/accept-terms", h.AuthAcceptTerms)
		r.Post("/auth/logout", h.AuthLogout)
		r.Get("/auth/sso/config", h.AuthSSOConfig)
		r.Post("/auth/sso/callback", h.AuthSSOCallback)

		// ── Public: Participant-facing endpoints ──
		r.Get("/survey/{surveyId}/public", h.GetPublicSurvey)
		r.Post("/survey/submit", h.SubmitParticipantSurvey)
		r.Get("/survey/{userId}", h.GetParticipantSurveyResponse)
		r.Get("/slots", h.GetAvailableSlots)

		// ── Public: Conference login (no JWT) ──
		r.Post("/conf/{confId}/login", h.ConferenceLogin)
		r.Get("/conf/{confId}/participants", h.GetConferenceParticipants)

		// ── Public: Webhooks / Callbacks (service-to-service, no JWT) ──
		// Recording upload callback from Conference Service (S3 trigger → Lambda → here)
		r.Post("/chime/recording/meeting/{meetingId}", h.RecordingUploadCallback)

		// ── Public: Interview media for conference participants (no JWT) ──
		r.Get("/interview_media/{confHash}/{mediaId}/pages/{page}/media.pdf", h.GetMediaPageForConference)

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
				r.Post("/project/create-project", h.CreateProjectMRA)
				r.Get("/project/get-project-details/{id}", h.GetProjectMRA)
				r.Post("/project/get-projects/client/{client_id}", h.ListProjectsMRA)
				r.Put("/project/update-project-details/{project_id}", h.UpdateProjectMRA)
				r.Put("/project/update-external-survey-id/{project_id}", h.UpdateExternalSurveyIDMRA)
				r.Post("/project/{project_id}/moderators_reset", h.ResetProjectModeratorsMRA)
				r.Get("/project/{project_id}/get_email_template", h.GetEmailTemplateMRA)
				r.Post("/project/{project_id}/handle-export", h.HandleProjectExportMRA)
				r.Put("/project/{project_id}/update-sample-size", h.UpdateSampleSizeMRA)
				r.Get("/project/{project_id}/get-unavailable-moderators", h.GetUnavailableModeratorsMRA)
				r.Post("/project/{project_id}/time_range/moderator/{moderator_id}", h.UpsertModeratorTimeRangeMRA)
				r.Get("/project/get-moderators-count/{project_id}/sample-size/{sample_size}", h.GetModeratorsCountMRA)
				r.Post("/projects/{client_id}/interviews", h.GetAllInterviewsMRA)
				r.Post("/interview/schedule", h.ScheduleInterviewMRA)
				r.Post("/interview/respondent_reschedule", h.RespondentRescheduleMRA)
				r.Post("/interview/invalidate", h.InvalidateInterviewMRA)
				r.Post("/interview/send_invalidate_reschedule_mail", h.SendInvalidateRescheduleMailMRA)
				r.Post("/add-conference-link/project/{project_id}/participant_group/{participant_group_id}", h.AddConferenceLinkMRA)
				r.Put("/update-conference-link/project/{project_id}/participant-group/{participant_group_id}", h.UpdateConferenceLinkMRA)
				r.Get("/get-conference-link/{participant_group_id}", h.GetConferenceLinkMRA)
				r.Get("/get-conf-link-time-slot-id/{timeslot_id}", h.GetConfLinkByTimeSlotMRA)
				r.Get("/user/get_all_moderators/{client_id}", h.GetAllModeratorsMRA)
				r.Post("/moderator/post-availability", h.PostModeratorAvailabilityMRA)
				r.Put("/moderator/{availability_id}/update-availability", h.UpdateModeratorAvailabilityMRA)
				r.Delete("/moderator/{availability_id}/delete-availability", h.DeleteModeratorAvailabilityMRA)
				r.Get("/get_moderators_availability/{qs_path}/survey/{survey_id}", h.GetModeratorsAvailabilityMRA)
				r.Post("/moderator/get/{moderator_id}/client/{client_id}/time_slots", h.GetModeratorTimeslotsMRA)
				r.Post("/moderator/get-all-interviews/{moderator_id}", h.GetModeratorInterviewsMRA)
				r.Get("/moderator/get-projects/{moderator_id}/client/{client_id}", h.GetProjectsForModeratorMRA)
				r.Get("/moderator/client/{project_id}/list", h.GetModeratorsListForProjectMRA)
			r.Put("/moderator/get/{moderator_id}", h.UpdateModeratorMRA)
				r.Get("/get-moderators-for-timeSlot/{timeslot_id}", h.GetModeratorsForTimeSlotMRA)
				r.Get("/get-moderators-option/{timeslot_id}", h.GetModeratorsOptionMRA)
				r.Get("/get-participant-id/{timeslot_id}", h.GetParticipantIdMRA)
				r.Get("/project_manager/client/{client_id}", h.GetAllProjectManagersMRA)
				r.Post("/project_manager/get/time_slots/client/{client_id}", h.GetPMTimeslotsMRA)
				r.Post("/project_manager/get/availabilities/client/{client_id}", h.GetAvailabilitiesForPMMRA)
				r.Get("/salesforce/getAllAccounts", h.GetAllAccountsMRA)
				r.Get("/salesforce/clients", h.GetSalesforceClientsMRA)
				r.Get("/salesforce/projects/{salesforce_client_id}", h.GetSalesforceProjectsMRA)
				r.Post("/salesforce/getSalesforceClientsWithFilter", h.GetSalesforceClientsWithFilterMRA)
				// MRA #61-66
				r.Post("/third-party-integrate", h.ThirdPartyIntegrateMRA)
				r.Post("/log-front-end-event", h.LogFrontEndEventMRA)
				r.Post("/qual/eligibility", h.QualEligibilityMRA)
				r.Get("/moderator/get/{moderator_id}/get-mod-av/{client_id}", h.GetModeratorAvailabilityByClientMRA)
				r.Post("/moderator/get/{moderator_id}/imported/{client_id}", h.StartModeratorImportMRA)
				r.Get("/moderator/get/{moderator_id}/imported-from-current-sync/{client_id}", h.GetImportedAvailabilityFromCurrentSyncMRA)
				// MRA #67-72
				r.Get("/moderator/get/{moderator_id}/import-status", h.GetImportStatusMRA)
				r.Post("/moderator/unlink-imp-mod/{moderator_id}", h.UnlinkImportedModeratorMRA)
				r.Get("/moderator/update-google-sheet-first-date", h.UpdateGoogleSheetFirstDateMRA)
				r.Get("/project/get_topics_by_project/{project_id}", h.GetTopicsByProjectMRA)
				r.Post("/update-topic-translation/{project_id}", h.UpdateTopicTranslationMRA)
				r.Delete("/translations/delete-topic-translation/{project_id}/{transaltion_to_delete}", h.DeleteTopicTranslationMRA)
				// MRA #73-78
				r.Get("/translations/get-all-localisations", h.GetAllLocalisationsMRA)
				r.Delete("/translations/delete-translation/{project_id}/{transaltion_to_delete}", h.DeleteTranslationMRA)
				r.Post("/add-honorarium-amount", h.AddHonorariumAmountMRA)
				r.Get("/hono-value-update-reason-list", h.GetHonoValueUpdateReasonListMRA)
				r.Post("/time-slot-payments", h.AddTimeSlotPaymentsMRA)
				r.Post("/time-slot-payments-external", h.AddExternalTimeSlotPaymentsMRA)
				// MRA #79-82
				r.Post("/time-slot-custom-hono", h.AddTimeSlotCustomHonorariumMRA)
				r.Get("/interview-payment-status-list", h.GetInterviewPaymentStatusListMRA)
				r.Get("/project/{id}", h.GetProject)
				r.Put("/project/{id}", h.UpdateProject)
				r.Delete("/project/{id}", h.DeleteProject)
				// Phase 6: Project sub-resources
				r.Get("/project/{id}/surveys", h.GetProjectSurveys)
				r.Get("/project/{id}/time_slots", h.GetProjectTimeSlots)
				r.Get("/project/{id}/users", h.GetProjectUsers)
				r.Get("/project/{pid}/observers", h.GetProjectObservers)
				r.Get("/project/{pid}/qual_resched_body", h.GetProjectQualReschedBody)
				r.Get("/project/{pid}/availability", h.GetProjectAvailability)
				r.Get("/project/{pid}/scheduler_moderators", h.GetProjectSchedulerModerators)
				r.Get("/project/{pid}/dashboard/availability_and_time_slots", h.GetProjectDashboard)
				r.Get("/project/{pid}/interview_media", h.GetProjectMedia)
				r.Get("/project/{pid}/interview_media/{mediaId}", h.GetProjectMediaDetail)
				r.Get("/project/{pid}/interview_media/{mediaId}/media.pdf", h.DownloadMediaPDF)
				r.Get("/project/{pid}/interview_media/{mediaId}/pages/{page}/img.png", h.DownloadMediaPage)
				r.Delete("/interview_media/{pid}/{mediaId}", h.DeleteProjectMedia)
			})

			// ── Surveys (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/surveys", h.ListSurveys)
				r.Post("/survey", h.CreateSurvey)
				r.Put("/survey/{id}", h.UpdateSurvey)
				r.Delete("/survey/{id}", h.DeleteSurvey)
				// Phase 6: Survey extended
				r.Get("/survey/{id}/detail", h.GetSurveyDetail)
				r.Get("/survey/{id}/validate", h.ValidateSurvey)
				r.Get("/survey/{id}/crowds", h.GetSurveyCrowds)
				r.Put("/survey/{id}/close", h.CloseSurvey)
				r.Put("/survey/{id}/favorite", h.ToggleSurveyFavorite)
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
				r.Delete("/moderators/{id}/availability/{availabilityId}", h.DeleteModeratorAvailability)
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
				// Phase 6: Subscription sub-resources
				r.Get("/subscription/{id}/interviews", h.GetSubscriptionInterviews)
				r.Get("/subscription/{id}/crowds", h.GetSubscriptionCrowds)
				r.Get("/subscription/{id}/question_types", h.GetSubscriptionQuestionTypes)
				r.Get("/subscription/{subId}/inquiries", h.GetSubscriptionInquiries)
				r.Get("/subscription/{subId}/project/{pid}/inquiry", h.GetSubscriptionProjectInquiry)
				r.Get("/subscription/{subId}/project_surveys", h.GetSubscriptionProjectSurveys)
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
				r.Post("/meeting", h.CreateMeeting)
				// Phase 6: Meeting extended
				r.Get("/meeting/metadata", h.GetMeetingMetadata)
				r.Put("/meeting/join/{joinId}", h.MeetingJoin)
				r.Get("/meeting/get_attendees_by_meeting_id/{meetingId}", h.GetAttendeesByMeetingID)
				r.Get("/meeting/recording_status/{meetingId}", h.GetRecordingStatus)
			})

			// ── Transcription (admin + manager) — CastingWords integration ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/transcription/order", h.CreateTranscriptionOrder)
				r.Get("/transcription/status/{orderId}", h.GetTranscriptionStatus)
				r.Get("/transcription/transcript/{orderId}", h.GetTranscript)
			})

			// ── SMS (admin + manager) — Bandwidth integration ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/sms/send", h.SendSMS)
			})

			// ── Payments (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/payments/timeslot", h.CreatePaymentReal)
				r.Post("/payments/custom-honorarium", h.CreateCustomHonorariumReal)
				r.Get("/payments/status-list", h.GetPaymentStatusListReal)
			})

			// ── Translations (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/translations/locales", h.GetLocalesReal)
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
				r.Get("/qstoolAdmin/get-all-users", h.ListAdminUsersMRA)
			})

			// ══════════════════════════════════════════
			// Phase 6 — Legacy API Routes
			// ══════════════════════════════════════════

			// ── Markets (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/markets", h.ListMarkets)
				r.Get("/markets/npi", h.ListMarketsNPI)
				r.Get("/market/{id}/crowdable_attributes", h.GetCrowdableAttributes)
			})

			// ── Timeslot Sub-resources (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/time_slot/{tsId}/moderators", h.GetTimeslotModerators)
				r.Get("/time_slot/{tsId}/moderators/options", h.GetTimeslotModeratorOptionsExt)
				r.Post("/time_slot/{tsId}/moderator", h.AssignTimeslotModerator)
				r.Delete("/time_slot/{tsId}/moderator/{modId}", h.UnassignTimeslotModerator)
				r.Get("/time_slot/{tsId}/observers", h.GetTimeslotObservers)
				r.Put("/time_slot/{tsId}/observers", h.UpdateTimeslotObservers)
				r.Post("/project/{pid}/time_slot/{tsId}/{action}", h.CancelRescheduleAction)
			})

			// ── Moderator Availability by subscription (admin + manager + moderator) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Get("/moderator/{modId}/availability/{subId}", h.GetModeratorAvailabilityBySub)
				r.Post("/moderator/{modId}/availability/{subId}", h.PostModeratorAvailabilityBySub)
				r.Put("/moderator/availability/{maId}", h.UpdateModeratorAvailabilityExt)
				r.Delete("/moderator/availability/{maId}", h.DeleteModeratorAvailabilityExt)
			})

			// ── Self-Service (admin + manager + moderator) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Get("/selfservice/noshow", h.GetNoShowCheck)
				r.Put("/selfservice/project/{pid}/timeslot/{tid}", h.MarkNoShow)
			})

			// ── User (authenticated) ──
			r.Get("/user/get-email/{id}", h.GetUserEmail)
			r.Get("/user/{id}", h.GetUser)
			r.Put("/user/{id}", h.UpdateUser)
			r.Post("/user/upsert-user-time-zone-selection", h.UpsertUserTimeZone)
			r.Put("/user/password_matches", h.CheckPasswordMatches)

			// ── Event Logs (any authenticated) ──
			r.Post("/EventLogs", h.CreateEventLog)

			// ── Salesforce Projects (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Get("/salesforceprojects", h.ListSalesforceProjects)
			})

			// ══════════════════════════════════════════
			// Phase 7 — Missing/Partial/Stub API Routes
			// ══════════════════════════════════════════

			// ── User Roles (admin) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin"))
				r.Post("/user/add_roles", h.AddUserRoles)
				r.Delete("/user/delete_roles", h.DeleteUserRoles)
			})

			// ── Password Reset (public-ish, but behind JWT) ──
			r.Post("/reset-user-password/send-user-password-email", h.SendPasswordResetEmail)
			r.Post("/reset-user-password/check-qsTool-i2", h.CheckUserIsQsToolAndI2)
			r.Patch("/reset-user-password/patch-user/{user_id}", h.PatchUser)
			r.Patch("/reset-user-password/patch-user-from-profile/{user_id}", h.PatchUserFromProfile)
			r.Put("/reset-user-password/check-if-password-matches/{user_id}", h.CheckPasswordMatchesMRA)

			// ── Unsubscribe / Comm Preferences (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/unsubscribe/check-user-comm-preference/{userId}", h.CheckUserCommPreference)
				r.Post("/unsubscribe/unsubscribe-user/{userId}", h.UnsubscribeUser)
			})

			// ── Project extensions (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/project/{projectId}/moderators_reset", h.ResetProjectModerators)
				r.Post("/project/{projectId}/handle-export", h.HandleProjectExport)
				r.Get("/project/{projectId}/available_moderators_count", h.GetAvailableModeratorsCount)
				r.Get("/project/{projectId}/unavailable_moderators", h.GetUnavailableModerators)
				r.Get("/project_manager/client", h.ListProjectManagers)
			})

			// ── Conference Links (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/project/{projectId}/add-conference-link", h.AddConferenceLink)
				r.Put("/update-conference-link", h.UpdateConferenceLinkHandler)
				r.Get("/conference-link/timeslot/{timeslotId}", h.GetConferenceLinkByTimeSlot)
			})

			// ── Notifications (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/notifications/send", h.SendNotificationEmail)
			})

			// ── Third-Party / Eligibility (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/third-party-integrate", h.ThirdPartyIntegrate)
				r.Post("/qual/eligibility", h.CheckQualEligibility)
			})

			// ── Translation Delete (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Delete("/projects/{projectId}/topics/translations/{translationKey}", h.DeleteTopicTranslation)
				r.Delete("/projects/{projectId}/translations/{langCode}", h.DeleteTranslation)
			})

			// ── External Payments (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Post("/payments/external", h.CreateExternalPayment)
				r.Get("/payments/honorarium-reasons", h.GetHonorariumReasons)
				r.Get("/payments/interview-statuses", h.GetInterviewPaymentStatusList)
			})

			// ── LS Inquiry (admin + manager) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager"))
				r.Put("/subscription/{subscriptionId}/inquiry_preview", h.UpdateInquiryPreview)
				r.Post("/custom_crowd_inquiry", h.CreateCustomCrowdInquiry)
			})

			// ── Google Calendar / Import placeholders (admin + manager + moderator) ──
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRoles("admin", "manager", "moderator"))
				r.Post("/moderator/{moderatorId}/import/start", h.StartModeratorImport)
				r.Get("/moderator/{moderatorId}/import/availability", h.GetImportedAvailability)
				r.Get("/moderator/{moderatorId}/import/status", h.GetImportStatus)
				r.Delete("/moderator/{moderatorId}/import/unlink", h.UnlinkImportedModerator)
				r.Put("/google-sheet/first-date", h.UpdateGoogleSheetFirstDate)
			})
		})
	})

	return r
}
