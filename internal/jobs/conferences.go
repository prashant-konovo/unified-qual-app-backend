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

	// TODO: List active Chime meetings via ConferenceClient.ListActiveMeetings()
	// The conference client currently has proxy methods for individual meeting operations
	// but may not have a ListActiveMeetings method.
	// When ConferenceClient.ListActiveMeetings is available:
	//
	// 1. activeMeetings, err := s.deps.Services.Conference.ListActiveMeetings(ctx)
	// 2. Extract conference hashes (friendly names) from active meetings
	// 3. staleHashes, err := s.deps.JobsRepo.GetStaleConferenceHashes(ctx, hashes)
	// 4. For each stale: s.deps.Services.Conference.EndMeeting(ctx, meetingID)

	_ = ctx
	_ = log

	// Placeholder — will be activated when ConferenceClient.ListActiveMeetings is available
}
