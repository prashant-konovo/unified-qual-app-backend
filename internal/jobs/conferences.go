package jobs

import (
	"context"
)

// endConferences terminates stale active conferences that have run past their
// scheduled end time by 30+ minutes.
// Matches Scala: QualInterviewManager.EndConferences (adapted from Twilio to Chime)
// Schedule: every 5 minutes
func (s *Scheduler) endConferences() {
	ctx := context.Background()
	log := s.logger.With("job", "EndConferences")

	if s.deps.JobsRepo == nil || s.deps.Services.Conference == nil {
		return
	}

	// ──────────────────────────────────────────────────────────────────
	// NOT YET IMPLEMENTED — missing infrastructure
	//
	// Scala (InCrowdAPI) uses Twilio SDK to discover active conferences:
	//   Conference.reader().setDateCreated(today).setStatus(IN_PROGRESS).read()
	// then cross-references with time_slot.conference_hash to find stale ones,
	// and ends them via Conference.updater(sid).setStatus(COMPLETED).update().
	//
	// Go backend uses AWS Chime (not Twilio). The ConferenceClient has
	// EndMeeting(meetingID) but lacks ListActiveMeetings() — the Chime
	// Lambda/API Gateway proxy does not expose a "list active meetings" endpoint.
	//
	// Recommended approach (no API Gateway changes required):
	//   1. Query time_slot WHERE chime_meeting_id IS NOT NULL
	//      AND end_time <= NOW() - INTERVAL 30 MINUTE AND status_id = 2
	//   2. For each stale meeting: ConferenceClient.EndMeeting(ctx, chimeMeetingID)
	//   3. Optionally update time_slot.status_id to reflect ended state
	//
	// Blocked on: Decision whether to query DB directly or add
	// ListActiveMeetings to the Chime API Gateway.
	// ──────────────────────────────────────────────────────────────────

	_ = ctx
	_ = log
}
