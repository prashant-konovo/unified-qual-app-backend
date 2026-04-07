package router

import (
	"github.com/go-chi/chi/v5"

	"github.com/InCrowd/unified-qual-api/internal/handler"
)

// ══════════════════════════════════════════════════════════════
// Conference routes — meetings, conference links, webhooks
// ══════════════════════════════════════════════════════════════

// registerPublicConferenceRoutes registers conference endpoints that do NOT require JWT.
func registerPublicConferenceRoutes(r chi.Router, hs *handler.Handlers) {
	r.Post("/conf/{confId}/login", hs.Handler.ConferenceLogin)
	r.Get("/conf/{confId}/participants", hs.Handler.GetConferenceParticipants)
	// Webhooks / Callbacks (service-to-service)
	r.Post("/chime/recording/meeting/{meetingId}", hs.Handler.RecordingUploadCallback)
	// Interview media for conference participants
	r.Get("/interview_media/{confHash}/{mediaId}/pages/{page}/media.pdf", hs.Handler.GetMediaPageForConference)
}

func registerConferenceRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Post("/meetings/{meetingId}/action/{action}", hs.Handler.MeetingAction)
		r.Post("/meeting/{meetingId}/universal", hs.Handler.MeetingUniversalJoin)
		r.Post("/meeting", hs.Handler.CreateMeeting)
		r.Get("/meeting/metadata", hs.Handler.GetMeetingMetadata)
		r.Put("/meeting/join/{joinId}", hs.Handler.MeetingJoin)
		r.Get("/meeting/get_attendees_by_meeting_id/{meetingId}", hs.Handler.GetAttendeesByMeetingID)
		r.Get("/meeting/recording_status/{meetingId}", hs.Handler.GetRecordingStatus)
	})

	// ── Conference links (admin + manager) ──
	r.Group(func(r chi.Router) {
		r.Use(adminManager)
		r.Post("/project/{projectId}/add-conference-link", hs.LS.AddConferenceLink)
		r.Put("/update-conference-link", hs.LS.UpdateConferenceLinkHandler)
		r.Get("/conference-link/timeslot/{timeslotId}", hs.LS.GetConferenceLinkByTimeSlot)
	})
}
