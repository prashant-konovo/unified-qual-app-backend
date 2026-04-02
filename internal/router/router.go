package router

import (
"github.com/InCrowd/unified-qual-api/internal/handler"
"github.com/InCrowd/unified-qual-api/internal/logger"
"github.com/InCrowd/unified-qual-api/internal/middleware"
"github.com/go-chi/chi/v5"
chimw "github.com/go-chi/chi/v5/middleware"
"github.com/go-chi/cors"
)

// Role presets — reusable middleware closures.
var (
adminOnly       = middleware.RequireRoles("admin")
adminManager    = middleware.RequireRoles("admin", "manager")
adminManagerMod = middleware.RequireRoles("admin", "manager", "moderator")
)

// New creates the chi router with all routes registered.
func New(hs *handler.Handlers, jwtAuth *middleware.JWTAuth) *chi.Mux {
r := chi.NewRouter()

// Global middleware
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

r.Get("/health", hs.Handler.Health)

r.Route("/v1", func(r chi.Router) {
registerPublicRoutes(r, hs)

// Protected: all remaining routes require valid JWT
r.Group(func(r chi.Router) {
r.Use(jwtAuth.Middleware)

registerAuthRoutes(r, hs)
registerProjectRoutes(r, hs)
registerSurveyRoutes(r, hs)
registerTimeslotRoutes(r, hs)
registerModeratorRoutes(r, hs)
registerInterviewRoutes(r, hs)
registerParticipantRoutes(r, hs)
registerBookingRoutes(r, hs)
registerSubscriptionRoutes(r, hs)
registerConferenceRoutes(r, hs)
registerAdminRoutes(r, hs)
registerLegacyRoutes(r, hs)
})
})

return r
}

// ──────────────────────────────────────────────
// Public routes (no JWT required)
// ──────────────────────────────────────────────

func registerPublicRoutes(r chi.Router, hs *handler.Handlers) {
// Auth
r.Post("/auth/login", hs.Auth.AuthLogin)
r.Post("/auth/magic-link", hs.Auth.AuthMagicLink)
r.Post("/auth/refresh", hs.Auth.AuthRefresh)
r.Post("/auth/accept-terms", hs.Auth.AuthAcceptTerms)
r.Post("/auth/logout", hs.Auth.AuthLogout)
r.Get("/auth/sso/config", hs.Auth.AuthSSOConfig)
r.Post("/auth/sso/callback", hs.Auth.AuthSSOCallback)

// Participant-facing
r.Get("/survey/{surveyId}/public", hs.Survey.GetPublicSurvey)
r.Post("/survey/submit", hs.Survey.SubmitParticipantSurvey)
r.Get("/survey/{userId}", hs.Survey.GetParticipantSurveyResponse)
r.Get("/slots", hs.Interview.GetAvailableSlots)

// Conference login
r.Post("/conf/{confId}/login", hs.Handler.ConferenceLogin)
r.Get("/conf/{confId}/participants", hs.Handler.GetConferenceParticipants)

// Webhooks / Callbacks
r.Post("/chime/recording/meeting/{meetingId}", hs.Handler.RecordingUploadCallback)

// Interview media for conference participants
r.Get("/interview_media/{confHash}/{mediaId}/pages/{page}/media.pdf", hs.Handler.GetMediaPageForConference)
}

// ──────────────────────────────────────────────
// Auth (protected)
// ──────────────────────────────────────────────

func registerAuthRoutes(r chi.Router, hs *handler.Handlers) {
r.Put("/auth/password", hs.Auth.AuthPassword)
r.Get("/auth/me", hs.Auth.AuthMe)
}

// ──────────────────────────────────────────────
// Projects
// ──────────────────────────────────────────────

func registerProjectRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/projects", hs.Project.ListProjects)
r.Post("/project", hs.Project.CreateProject)
// MRA project routes
r.Post("/project/create-project", hs.Handler.CreateProjectMRA)
r.Get("/project/get-project-details/{id}", hs.Handler.GetProjectMRA)
r.Post("/project/get-projects/client/{client_id}", hs.Handler.ListProjectsMRA)
r.Put("/project/update-project-details/{project_id}", hs.Handler.UpdateProjectMRA)
r.Put("/project/update-external-survey-id/{project_id}", hs.Handler.UpdateExternalSurveyIDMRA)
r.Post("/project/{project_id}/moderators_reset", hs.Handler.ResetProjectModeratorsMRA)
r.Get("/project/{project_id}/get_email_template", hs.Handler.GetEmailTemplateMRA)
r.Post("/project/{project_id}/handle-export", hs.Handler.HandleProjectExportMRA)
r.Put("/project/{project_id}/update-sample-size", hs.Handler.UpdateSampleSizeMRA)
r.Get("/project/{project_id}/get-unavailable-moderators", hs.Handler.GetUnavailableModeratorsMRA)
r.Post("/project/{project_id}/time_range/moderator/{moderator_id}", hs.Handler.UpsertModeratorTimeRangeMRA)
r.Get("/project/get-moderators-count/{project_id}/sample-size/{sample_size}", hs.Handler.GetModeratorsCountMRA)
r.Post("/projects/{client_id}/interviews", hs.Handler.GetAllInterviewsMRA)
r.Post("/interview/schedule", hs.Handler.ScheduleInterviewMRA)
r.Post("/interview/respondent_reschedule", hs.Handler.RespondentRescheduleMRA)
r.Post("/interview/invalidate", hs.Handler.InvalidateInterviewMRA)
r.Post("/interview/send_invalidate_reschedule_mail", hs.Handler.SendInvalidateRescheduleMailMRA)
r.Post("/add-conference-link/project/{project_id}/participant_group/{participant_group_id}", hs.Handler.AddConferenceLinkMRA)
r.Put("/update-conference-link/project/{project_id}/participant-group/{participant_group_id}", hs.Handler.UpdateConferenceLinkMRA)
r.Get("/get-conference-link/{participant_group_id}", hs.Handler.GetConferenceLinkMRA)
r.Get("/get-conf-link-time-slot-id/{timeslot_id}", hs.Handler.GetConfLinkByTimeSlotMRA)
r.Get("/user/get_all_moderators/{client_id}", hs.Handler.GetAllModeratorsMRA)
r.Post("/moderator/post-availability", hs.Handler.PostModeratorAvailabilityMRA)
r.Put("/moderator/{availability_id}/update-availability", hs.Handler.UpdateModeratorAvailabilityMRA)
r.Delete("/moderator/{availability_id}/delete-availability", hs.Handler.DeleteModeratorAvailabilityMRA)
r.Get("/get_moderators_availability/{qs_path}/survey/{survey_id}", hs.Handler.GetModeratorsAvailabilityMRA)
r.Post("/moderator/get/{moderator_id}/client/{client_id}/time_slots", hs.Handler.GetModeratorTimeslotsMRA)
r.Post("/moderator/get-all-interviews/{moderator_id}", hs.Handler.GetModeratorInterviewsMRA)
r.Get("/moderator/get-projects/{moderator_id}/client/{client_id}", hs.Handler.GetProjectsForModeratorMRA)
r.Get("/moderator/client/{project_id}/list", hs.Handler.GetModeratorsListForProjectMRA)
r.Put("/moderator/get/{moderator_id}", hs.Handler.UpdateModeratorMRA)
r.Get("/get-moderators-for-timeSlot/{timeslot_id}", hs.Handler.GetModeratorsForTimeSlotMRA)
r.Get("/get-moderators-option/{timeslot_id}", hs.Handler.GetModeratorsOptionMRA)
r.Get("/get-participant-id/{timeslot_id}", hs.Handler.GetParticipantIdMRA)
r.Get("/project_manager/client/{client_id}", hs.Handler.GetAllProjectManagersMRA)
r.Post("/project_manager/get/time_slots/client/{client_id}", hs.Handler.GetPMTimeslotsMRA)
r.Post("/project_manager/get/availabilities/client/{client_id}", hs.Handler.GetAvailabilitiesForPMMRA)
r.Get("/salesforce/getAllAccounts", hs.Handler.GetAllAccountsMRA)
r.Get("/salesforce/clients", hs.Handler.GetSalesforceClientsMRA)
r.Get("/salesforce/projects/{salesforce_client_id}", hs.Handler.GetSalesforceProjectsMRA)
r.Post("/salesforce/getSalesforceClientsWithFilter", hs.Handler.GetSalesforceClientsWithFilterMRA)
// MRA #61-66
r.Post("/third-party-integrate", hs.Handler.ThirdPartyIntegrateMRA)
r.Post("/log-front-end-event", hs.Handler.LogFrontEndEventMRA)
r.Post("/qual/eligibility", hs.Handler.QualEligibilityMRA)
r.Get("/moderator/get/{moderator_id}/get-mod-av/{client_id}", hs.Handler.GetModeratorAvailabilityByClientMRA)
r.Post("/moderator/get/{moderator_id}/imported/{client_id}", hs.Handler.StartModeratorImportMRA)
r.Get("/moderator/get/{moderator_id}/imported-from-current-sync/{client_id}", hs.Handler.GetImportedAvailabilityFromCurrentSyncMRA)
// MRA #67-72
r.Get("/moderator/get/{moderator_id}/import-status", hs.Handler.GetImportStatusMRA)
r.Post("/moderator/unlink-imp-mod/{moderator_id}", hs.Handler.UnlinkImportedModeratorMRA)
r.Get("/moderator/update-google-sheet-first-date", hs.Handler.UpdateGoogleSheetFirstDateMRA)
r.Get("/project/get_topics_by_project/{project_id}", hs.Handler.GetTopicsByProjectMRA)
r.Post("/update-topic-translation/{project_id}", hs.Handler.UpdateTopicTranslationMRA)
r.Delete("/translations/delete-topic-translation/{project_id}/{transaltion_to_delete}", hs.Handler.DeleteTopicTranslationMRA)
// MRA #73-78
r.Get("/translations/get-all-localisations", hs.Handler.GetAllLocalisationsMRA)
r.Delete("/translations/delete-translation/{project_id}/{transaltion_to_delete}", hs.Handler.DeleteTranslationMRA)
r.Post("/add-honorarium-amount", hs.Handler.AddHonorariumAmountMRA)
r.Get("/hono-value-update-reason-list", hs.Handler.GetHonoValueUpdateReasonListMRA)
r.Post("/time-slot-payments", hs.Handler.AddTimeSlotPaymentsMRA)
r.Post("/time-slot-payments-external", hs.Handler.AddExternalTimeSlotPaymentsMRA)
// MRA #79-82
r.Post("/time-slot-custom-hono", hs.Handler.AddTimeSlotCustomHonorariumMRA)
r.Get("/interview-payment-status-list", hs.Handler.GetInterviewPaymentStatusListMRA)
// Core project CRUD
r.Get("/project/{id}", hs.Project.GetProject)
r.Put("/project/{id}", hs.Project.UpdateProject)
r.Delete("/project/{id}", hs.Project.DeleteProject)
// Project sub-resources
r.Get("/project/{id}/surveys", hs.Handler.GetProjectSurveys)
r.Get("/project/{id}/time_slots", hs.Handler.GetProjectTimeSlots)
r.Get("/project/{id}/users", hs.Handler.GetProjectUsers)
r.Get("/project/{pid}/observers", hs.Handler.GetProjectObservers)
r.Get("/project/{pid}/qual_resched_body", hs.Handler.GetProjectQualReschedBody)
r.Get("/project/{pid}/availability", hs.Handler.GetProjectAvailability)
r.Get("/project/{pid}/scheduler_moderators", hs.Handler.GetProjectSchedulerModerators)
r.Get("/project/{pid}/dashboard/availability_and_time_slots", hs.Handler.GetProjectDashboard)
r.Get("/project/{pid}/interview_media", hs.Handler.GetProjectMedia)
r.Get("/project/{pid}/interview_media/{mediaId}", hs.Handler.GetProjectMediaDetail)
r.Get("/project/{pid}/interview_media/{mediaId}/media.pdf", hs.Handler.DownloadMediaPDF)
r.Get("/project/{pid}/interview_media/{mediaId}/pages/{page}/img.png", hs.Handler.DownloadMediaPage)
r.Delete("/interview_media/{pid}/{mediaId}", hs.Handler.DeleteProjectMedia)
})
}

// ──────────────────────────────────────────────
// Surveys
// ──────────────────────────────────────────────

func registerSurveyRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/surveys", hs.Survey.ListSurveys)
r.Post("/survey", hs.Survey.CreateSurvey)
r.Put("/survey/{id}", hs.Survey.UpdateSurvey)
r.Delete("/survey/{id}", hs.Survey.DeleteSurvey)
// Survey extended
r.Get("/survey/{id}/detail", hs.Handler.GetSurveyDetail)
r.Get("/survey/{id}/validate", hs.Handler.ValidateSurvey)
r.Get("/survey/{id}/crowds", hs.Handler.GetSurveyCrowds)
r.Put("/survey/{id}/close", hs.Handler.CloseSurvey)
r.Put("/survey/{id}/favorite", hs.Handler.ToggleSurveyFavorite)
})

// Survey Responses (any authenticated user)
r.Get("/survey-responses/{userId}", hs.Survey.GetSurveyResponses)
r.Post("/survey-responses", hs.Survey.SubmitSurveyResponse)
}

// ──────────────────────────────────────────────
// Timeslots
// ──────────────────────────────────────────────

func registerTimeslotRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/timeslots", hs.Interview.ListTimeslots)
r.Post("/timeslots", hs.Interview.CreateTimeslot)
r.Get("/timeslots/{id}", hs.Interview.GetTimeslot)
r.Put("/timeslots/{id}", hs.Interview.UpdateTimeslot)
r.Delete("/timeslots/{id}", hs.Interview.DeleteTimeslot)
})

// Interview Slots
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/slots/generate", hs.Interview.GenerateSlots)
r.Get("/ai/suggested-slots", hs.Interview.GetAISuggestions)
})
}

// ──────────────────────────────────────────────
// Moderators
// ──────────────────────────────────────────────

func registerModeratorRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/moderators", hs.Handler.ListModerators)
r.Post("/moderators", hs.Handler.CreateModerator)
r.Get("/moderators/{id}", hs.Handler.GetModerator)
r.Put("/moderators/{id}", hs.Handler.UpdateModerator)
r.Delete("/moderators/{id}", hs.Handler.DeleteModerator)
r.Post("/moderators/bulk-upload", hs.Handler.BulkUploadModerators)
})
// Moderator availability — admin, manager, or moderator
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Get("/moderators/{id}/availability", hs.Handler.GetModeratorAvailability)
r.Post("/moderators/{id}/availability", hs.Handler.PostModeratorAvailability)
r.Delete("/moderators/{id}/availability/{availabilityId}", hs.Handler.DeleteModeratorAvailability)
r.Get("/moderators/{moderatorId}/timeslots", hs.Handler.GetModeratorTimeslots)
r.Get("/timeslots/{id}/moderators/options", hs.Handler.GetTimeslotModeratorOptions)
})
}

// ──────────────────────────────────────────────
// Interviews
// ──────────────────────────────────────────────

func registerInterviewRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Post("/interviews/schedule", hs.Handler.ScheduleInterview)
r.Post("/interviews/{id}/cancel", hs.Handler.CancelInterview)
r.Post("/interviews/{id}/reschedule", hs.Handler.RescheduleInterview)
})
}

// ──────────────────────────────────────────────
// Participants
// ──────────────────────────────────────────────

func registerParticipantRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/participants", hs.Handler.ListParticipants)
r.Post("/participants", hs.Handler.CreateParticipant)
r.Get("/participants/{id}", hs.Handler.GetParticipant)
})
}

// ──────────────────────────────────────────────
// Bookings
// ──────────────────────────────────────────────

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

// ──────────────────────────────────────────────
// Subscriptions
// ──────────────────────────────────────────────

func registerSubscriptionRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/subscriptions", hs.Handler.ListSubscriptions)
r.Post("/subscription", hs.Handler.CreateSubscription)
r.Get("/subscription/{id}", hs.Handler.GetSubscription)
r.Put("/subscription/{id}", hs.Handler.UpdateSubscription)
r.Delete("/subscription/{id}", hs.Handler.DeleteSubscription)
// Subscription sub-resources
r.Get("/subscription/{id}/interviews", hs.Handler.GetSubscriptionInterviews)
r.Get("/subscription/{id}/crowds", hs.Handler.GetSubscriptionCrowds)
r.Get("/subscription/{id}/question_types", hs.Handler.GetSubscriptionQuestionTypes)
r.Get("/subscription/{subId}/inquiries", hs.Handler.GetSubscriptionInquiries)
r.Get("/subscription/{subId}/project/{pid}/inquiry", hs.Handler.GetSubscriptionProjectInquiry)
r.Get("/subscription/{subId}/project_surveys", hs.Handler.GetSubscriptionProjectSurveys)
})

// Waiting Queue
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/waiting-queue", hs.Handler.GetWaitingQueue)
r.Post("/waiting-queue", hs.Handler.AddToWaitingQueue)
r.Delete("/waiting-queue/{id}", hs.Handler.RemoveFromWaitingQueue)
r.Post("/match-slots", hs.Handler.TriggerMatching)
})
}

// ──────────────────────────────────────────────
// Conference
// ──────────────────────────────────────────────

func registerConferenceRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Post("/meetings/{meetingId}/action/{action}", hs.Handler.MeetingAction)
r.Post("/meeting/{meetingId}/universal", hs.Handler.MeetingUniversalJoin)
r.Post("/meeting", hs.Handler.CreateMeeting)
// Meeting extended
r.Get("/meeting/metadata", hs.Handler.GetMeetingMetadata)
r.Put("/meeting/join/{joinId}", hs.Handler.MeetingJoin)
r.Get("/meeting/get_attendees_by_meeting_id/{meetingId}", hs.Handler.GetAttendeesByMeetingID)
r.Get("/meeting/recording_status/{meetingId}", hs.Handler.GetRecordingStatus)
})

// Transcription
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/transcription/order", hs.Handler.CreateTranscriptionOrder)
r.Get("/transcription/status/{orderId}", hs.Handler.GetTranscriptionStatus)
r.Get("/transcription/transcript/{orderId}", hs.Handler.GetTranscript)
})

// SMS
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/sms/send", hs.Handler.SendSMS)
})
}

// ──────────────────────────────────────────────
// Admin
// ──────────────────────────────────────────────

func registerAdminRoutes(r chi.Router, hs *handler.Handlers) {
r.Group(func(r chi.Router) {
r.Use(adminOnly)
r.Get("/admin/users", hs.Handler.ListAdminUsers)
r.Post("/admin/users", hs.Handler.CreateAdminUser)
r.Get("/qstoolAdmin/get-all-users", hs.Handler.ListAdminUsersMRA)
})
}

// ──────────────────────────────────────────────
// Legacy routes (Phase 6/7)
// ──────────────────────────────────────────────

func registerLegacyRoutes(r chi.Router, hs *handler.Handlers) {
// Markets
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/markets", hs.Handler.ListMarkets)
r.Get("/markets/npi", hs.Handler.ListMarketsNPI)
r.Get("/market/{id}/crowdable_attributes", hs.Handler.GetCrowdableAttributes)
})

// Timeslot Sub-resources
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/time_slot/{tsId}/moderators", hs.Handler.GetTimeslotModerators)
r.Get("/time_slot/{tsId}/moderators/options", hs.Handler.GetTimeslotModeratorOptionsExt)
r.Post("/time_slot/{tsId}/moderator", hs.Handler.AssignTimeslotModerator)
r.Delete("/time_slot/{tsId}/moderator/{modId}", hs.Handler.UnassignTimeslotModerator)
r.Get("/time_slot/{tsId}/observers", hs.Handler.GetTimeslotObservers)
r.Put("/time_slot/{tsId}/observers", hs.Handler.UpdateTimeslotObservers)
r.Post("/project/{pid}/time_slot/{tsId}/{action}", hs.Handler.CancelRescheduleAction)
})

// Moderator Availability by subscription
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Get("/moderator/{modId}/availability/{subId}", hs.Handler.GetModeratorAvailabilityBySub)
r.Post("/moderator/{modId}/availability/{subId}", hs.Handler.PostModeratorAvailabilityBySub)
r.Put("/moderator/availability/{maId}", hs.Handler.UpdateModeratorAvailabilityExt)
r.Delete("/moderator/availability/{maId}", hs.Handler.DeleteModeratorAvailabilityExt)
})

// Self-Service
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Get("/selfservice/noshow", hs.Handler.GetNoShowCheck)
r.Put("/selfservice/project/{pid}/timeslot/{tid}", hs.Handler.MarkNoShow)
})

// User (authenticated)
r.Get("/user/get-email/{id}", hs.Handler.GetUserEmail)
r.Get("/user/{id}", hs.Handler.GetUser)
r.Put("/user/{id}", hs.Handler.UpdateUser)
r.Post("/user/upsert-user-time-zone-selection", hs.Handler.UpsertUserTimeZone)
r.Put("/user/password_matches", hs.Handler.CheckPasswordMatches)

// Event Logs
r.Post("/EventLogs", hs.Handler.CreateEventLog)

// Salesforce Projects
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/salesforceprojects", hs.Handler.ListSalesforceProjects)
})

// Payments
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/payments/timeslot", hs.Handler.CreatePaymentReal)
r.Post("/payments/custom-honorarium", hs.Handler.CreateCustomHonorariumReal)
r.Get("/payments/status-list", hs.Handler.GetPaymentStatusListReal)
})

// Translations
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/translations/locales", hs.Handler.GetLocalesReal)
r.Put("/projects/{projectId}/topics/translations", hs.Handler.UpdateTopicTranslations)
})

// Notifications
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Get("/notifications/email-template", hs.Handler.GetEmailTemplate)
r.Post("/notifications/reminder", hs.Handler.SendReminder)
r.Post("/notifications/send", hs.Handler.SendNotificationEmail)
})

// User Roles (admin)
r.Group(func(r chi.Router) {
r.Use(adminOnly)
r.Post("/user/add_roles", hs.Handler.AddUserRoles)
r.Delete("/user/delete_roles", hs.Handler.DeleteUserRoles)
})

// Password Reset
r.Post("/reset-user-password/send-user-password-email", hs.Handler.SendPasswordResetEmail)
r.Post("/reset-user-password/check-qsTool-i2", hs.Handler.CheckUserIsQsToolAndI2)
r.Patch("/reset-user-password/patch-user/{user_id}", hs.Handler.PatchUser)
r.Patch("/reset-user-password/patch-user-from-profile/{user_id}", hs.Handler.PatchUserFromProfile)
r.Put("/reset-user-password/check-if-password-matches/{user_id}", hs.Handler.CheckPasswordMatchesMRA)

// Unsubscribe / Comm Preferences
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/unsubscribe/check-user-comm-preference/{userId}", hs.Handler.CheckUserCommPreference)
r.Post("/unsubscribe/unsubscribe-user/{userId}", hs.Handler.UnsubscribeUser)
})

// Project extensions
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/project/{projectId}/moderators_reset", hs.Handler.ResetProjectModerators)
r.Post("/project/{projectId}/handle-export", hs.Handler.HandleProjectExport)
r.Get("/project/{projectId}/available_moderators_count", hs.Handler.GetAvailableModeratorsCount)
r.Get("/project/{projectId}/unavailable_moderators", hs.Handler.GetUnavailableModerators)
r.Get("/project_manager/client", hs.Handler.ListProjectManagers)
})

// Conference Links
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/project/{projectId}/add-conference-link", hs.Handler.AddConferenceLink)
r.Put("/update-conference-link", hs.Handler.UpdateConferenceLinkHandler)
r.Get("/conference-link/timeslot/{timeslotId}", hs.Handler.GetConferenceLinkByTimeSlot)
})

// Third-Party / Eligibility
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/third-party-integrate", hs.Handler.ThirdPartyIntegrate)
r.Post("/qual/eligibility", hs.Handler.CheckQualEligibility)
})

// Translation Delete
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Delete("/projects/{projectId}/topics/translations/{translationKey}", hs.Handler.DeleteTopicTranslation)
r.Delete("/projects/{projectId}/translations/{langCode}", hs.Handler.DeleteTranslation)
})

// External Payments
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Post("/payments/external", hs.Handler.CreateExternalPayment)
r.Get("/payments/honorarium-reasons", hs.Handler.GetHonorariumReasons)
r.Get("/payments/interview-statuses", hs.Handler.GetInterviewPaymentStatusList)
})

// LS Inquiry
r.Group(func(r chi.Router) {
r.Use(adminManager)
r.Put("/subscription/{subscriptionId}/inquiry_preview", hs.Handler.UpdateInquiryPreview)
r.Post("/custom_crowd_inquiry", hs.Handler.CreateCustomCrowdInquiry)
})

// Google Calendar / Import placeholders
r.Group(func(r chi.Router) {
r.Use(adminManagerMod)
r.Post("/moderator/{moderatorId}/import/start", hs.Handler.StartModeratorImport)
r.Get("/moderator/{moderatorId}/import/availability", hs.Handler.GetImportedAvailability)
r.Get("/moderator/{moderatorId}/import/status", hs.Handler.GetImportStatus)
r.Delete("/moderator/{moderatorId}/import/unlink", hs.Handler.UnlinkImportedModerator)
r.Put("/google-sheet/first-date", hs.Handler.UpdateGoogleSheetFirstDate)
})
}
