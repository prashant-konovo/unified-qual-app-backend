package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Project routes — includes MRA variants and sub-resources
// ══════════════════════════════════════════════════════════════

func registerProjectRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Get("/projects", hs.Project.ListProjects)
		r.Post("/project", hs.Project.CreateProject)
		r.Post("/project/create-project", hs.MRA.CreateProjectMRA)
		r.Get("/project/get-project-details/{id}", hs.MRA.GetProjectMRA)
		r.Post("/project/get-projects/client/{client_id}", hs.MRA.ListProjectsMRA)
		r.Put("/project/update-project-details/{project_id}", hs.MRA.UpdateProjectMRA)
		r.Put("/project/update-external-survey-id/{project_id}", hs.MRA.UpdateExternalSurveyIDMRA)
		r.Post("/project/{project_id}/moderators_reset", hs.MRA.ResetProjectModeratorsMRA)
		r.Get("/project/{project_id}/get_email_template", hs.MRA.GetEmailTemplateMRA)
		r.Post("/project/{project_id}/handle-export", hs.MRA.HandleProjectExportMRA)
		r.Put("/project/{project_id}/update-sample-size", hs.MRA.UpdateSampleSizeMRA)
		r.Get("/project/{project_id}/get-unavailable-moderators", hs.MRA.GetUnavailableModeratorsMRA)
		r.Post("/project/{project_id}/time_range/moderator/{moderator_id}", hs.MRA.UpsertModeratorTimeRangeMRA)
		r.Get("/project/get-moderators-count/{project_id}/sample-size/{sample_size}", hs.MRA.GetModeratorsCountMRA)
		r.Post("/projects/{client_id}/interviews", hs.MRA.GetAllInterviewsMRA)
		r.Post("/interview/schedule", hs.MRA.ScheduleInterviewMRA)
		r.Post("/interview/respondent_reschedule", hs.MRA.RespondentRescheduleMRA)
		r.Post("/interview/invalidate", hs.MRA.InvalidateInterviewMRA)
		r.Post("/interview/send_invalidate_reschedule_mail", hs.MRA.SendInvalidateRescheduleMailMRA)
		r.Post("/add-conference-link/project/{project_id}/participant_group/{participant_group_id}", hs.MRA.AddConferenceLinkMRA)
		r.Put("/update-conference-link/project/{project_id}/participant-group/{participant_group_id}", hs.MRA.UpdateConferenceLinkMRA)
		r.Get("/get-conference-link/{participant_group_id}", hs.MRA.GetConferenceLinkMRA)
		r.Get("/get-conf-link-time-slot-id/{timeslot_id}", hs.MRA.GetConfLinkByTimeSlotMRA)
		r.Get("/user/get_all_moderators/{client_id}", hs.MRA.GetAllModeratorsMRA)
		r.Post("/moderator/post-availability", hs.MRA.PostModeratorAvailabilityMRA)
		r.Put("/moderator/{availability_id}/update-availability", hs.MRA.UpdateModeratorAvailabilityMRA)
		r.Delete("/moderator/{availability_id}/delete-availability", hs.MRA.DeleteModeratorAvailabilityMRA)
		r.Get("/get_moderators_availability/{qs_path}/survey/{survey_id}", hs.MRA.GetModeratorsAvailabilityMRA)
		r.Post("/moderator/get/{moderator_id}/client/{client_id}/time_slots", hs.MRA.GetModeratorTimeslotsMRA)
		r.Post("/moderator/get-all-interviews/{moderator_id}", hs.MRA.GetModeratorInterviewsMRA)
		r.Get("/moderator/get-projects/{moderator_id}/client/{client_id}", hs.MRA.GetProjectsForModeratorMRA)
		r.Get("/moderator/client/{project_id}/list", hs.MRA.GetModeratorsListForProjectMRA)
		r.Put("/moderator/get/{moderator_id}", hs.MRA.UpdateModeratorMRA)
		r.Get("/get-moderators-for-timeSlot/{timeslot_id}", hs.MRA.GetModeratorsForTimeSlotMRA)
		r.Get("/get-moderators-option/{timeslot_id}", hs.MRA.GetModeratorsOptionMRA)
		r.Get("/get-participant-id/{timeslot_id}", hs.MRA.GetParticipantIdMRA)
		r.Get("/project_manager/client/{client_id}", hs.MRA.GetAllProjectManagersMRA)
		r.Post("/project_manager/get/time_slots/client/{client_id}", hs.MRA.GetPMTimeslotsMRA)
		r.Post("/project_manager/get/availabilities/client/{client_id}", hs.MRA.GetAvailabilitiesForPMMRA)
		r.Get("/salesforce/getAllAccounts", hs.MRA.GetAllAccountsMRA)
		r.Get("/salesforce/clients", hs.MRA.GetSalesforceClientsMRA)
		r.Get("/salesforce/projects/{salesforce_client_id}", hs.MRA.GetSalesforceProjectsMRA)
		r.Post("/salesforce/getSalesforceClientsWithFilter", hs.MRA.GetSalesforceClientsWithFilterMRA)
		// MRA #61-66
		r.Post("/third-party-integrate", hs.MRA.ThirdPartyIntegrateMRA)
		r.Post("/log-front-end-event", hs.MRA.LogFrontEndEventMRA)
		r.Post("/qual/eligibility", hs.MRA.QualEligibilityMRA)
		r.Get("/moderator/get/{moderator_id}/get-mod-av/{client_id}", hs.MRA.GetModeratorAvailabilityByClientMRA)
		r.Post("/moderator/get/{moderator_id}/imported/{client_id}", hs.MRA.StartModeratorImportMRA)
		r.Get("/moderator/get/{moderator_id}/imported-from-current-sync/{client_id}", hs.MRA.GetImportedAvailabilityFromCurrentSyncMRA)
		// MRA #67-72
		r.Get("/moderator/get/{moderator_id}/import-status", hs.MRA.GetImportStatusMRA)
		r.Post("/moderator/unlink-imp-mod/{moderator_id}", hs.MRA.UnlinkImportedModeratorMRA)
		r.Get("/moderator/update-google-sheet-first-date", hs.MRA.UpdateGoogleSheetFirstDateMRA)
		r.Get("/project/get_topics_by_project/{project_id}", hs.MRA.GetTopicsByProjectMRA)
		r.Post("/update-topic-translation/{project_id}", hs.MRA.UpdateTopicTranslationMRA)
		r.Delete("/translations/delete-topic-translation/{project_id}/{transaltion_to_delete}", hs.MRA.DeleteTopicTranslationMRA)
		// MRA #73-78
		r.Get("/translations/get-all-localisations", hs.MRA.GetAllLocalisationsMRA)
		r.Delete("/translations/delete-translation/{project_id}/{transaltion_to_delete}", hs.MRA.DeleteTranslationMRA)
		r.Post("/add-honorarium-amount", hs.MRA.AddHonorariumAmountMRA)
		r.Get("/hono-value-update-reason-list", hs.MRA.GetHonoValueUpdateReasonListMRA)
		r.Post("/time-slot-payments", hs.MRA.AddTimeSlotPaymentsMRA)
		r.Post("/time-slot-payments-external", hs.MRA.AddExternalTimeSlotPaymentsMRA)
		// MRA #79-82
		r.Post("/time-slot-custom-hono", hs.MRA.AddTimeSlotCustomHonorariumMRA)
		r.Get("/interview-payment-status-list", hs.MRA.GetInterviewPaymentStatusListMRA)
		r.Get("/project/{id}", hs.Project.GetProject)
		r.Put("/project/{id}", hs.Project.UpdateProject)
		r.Delete("/project/{id}", hs.Project.DeleteProject)
		// Phase 6: Project sub-resources
		r.Get("/project/{id}/surveys", hs.LS.GetProjectSurveys)
		r.Get("/project/{id}/time_slots", hs.LS.GetProjectTimeSlots)
		r.Get("/project/{id}/users", hs.LS.GetProjectUsers)
		r.Get("/project/{pid}/observers", hs.LS.GetProjectObservers)
		r.Get("/project/{pid}/qual_resched_body", hs.LS.GetProjectQualReschedBody)
		r.Get("/project/{pid}/availability", hs.LS.GetProjectAvailability)
		r.Get("/project/{pid}/scheduler_moderators", hs.LS.GetProjectSchedulerModerators)
		r.Get("/project/{pid}/dashboard/availability_and_time_slots", hs.LS.GetProjectDashboard)
		r.Get("/project/{pid}/interview_media", hs.Handler.GetProjectMedia)
		r.Get("/project/{pid}/interview_media/{mediaId}", hs.Handler.GetProjectMediaDetail)
		r.Get("/project/{pid}/interview_media/{mediaId}/media.pdf", hs.Handler.DownloadMediaPDF)
		r.Get("/project/{pid}/interview_media/{mediaId}/pages/{page}/img.png", hs.Handler.DownloadMediaPage)
		r.Delete("/interview_media/{pid}/{mediaId}", hs.Handler.DeleteProjectMedia)
	})

	// ── Project extensions (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/project/{projectId}/moderators_reset", hs.LS.ResetProjectModerators)
		r.Post("/project/{projectId}/handle-export", hs.LS.HandleProjectExport)
		r.Get("/project/{projectId}/available_moderators_count", hs.LS.GetAvailableModeratorsCount)
		r.Get("/project/{projectId}/unavailable_moderators", hs.LS.GetUnavailableModerators)
		r.Get("/project_manager/client", hs.LS.ListProjectManagers)
		r.Get("/salesforceprojects", hs.Handler.ListSalesforceProjects)
	})

	// ── Translations (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Put("/projects/{projectId}/topics/translations", hs.Handler.UpdateTopicTranslations)
		r.Delete("/projects/{projectId}/topics/translations/{translationKey}", hs.LS.DeleteTopicTranslation)
		r.Delete("/projects/{projectId}/translations/{langCode}", hs.LS.DeleteTranslation)
	})
}
