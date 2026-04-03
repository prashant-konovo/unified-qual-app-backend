package router

import (
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/go-chi/chi/v5"
)

// ══════════════════════════════════════════════════════════════
// Conference routes — meetings
// ══════════════════════════════════════════════════════════════

func registerConferenceRoutes(r chi.Router, hs *handler.Handlers) {
	r.Group(func(r chi.Router) {
		r.Use(adminManagerMod)
		r.Post("/meetings/{meetingId}/action/{action}", hs.Handler.MeetingAction)
		r.Post("/meeting/{meetingId}/universal", hs.Handler.MeetingUniversalJoin)
		r.Post("/meeting", hs.Handler.CreateMeeting)
		// Phase 6: Meeting extended
		r.Get("/meeting/metadata", hs.Handler.GetMeetingMetadata)
		r.Put("/meeting/join/{joinId}", hs.Handler.MeetingJoin)
		r.Get("/meeting/get_attendees_by_meeting_id/{meetingId}", hs.Handler.GetAttendeesByMeetingID)
		r.Get("/meeting/recording_status/{meetingId}", hs.Handler.GetRecordingStatus)
	})
}
