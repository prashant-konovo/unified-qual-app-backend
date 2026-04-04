package jobs

import (
	"context"
)

// remindToConfirmSchedule nudges unconfirmed interviewees by triggering a
// Google Calendar notification on their scheduled events.
// Matches Scala: CommsManager.RemindToConfirmSchedule
// Schedule: daily at 11am UTC
func (s *Scheduler) remindToConfirmSchedule() {
	ctx := context.Background()
	log := s.logger.With("job", "RemindToConfirmSchedule")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	answers, err := s.deps.JobsRepo.GetUnconfirmedAnswers(ctx)
	if err != nil {
		log.Error("failed to get unconfirmed answers", "error", err)
		return
	}
	if len(answers) == 0 {
		log.Info("no unconfirmed answers found")
		return
	}

	log.Info("found unconfirmed answers", "count", len(answers))

	gcal := s.deps.Services.GoogleCal
	if gcal == nil || !gcal.Configured() {
		log.Warn("Google Calendar client not configured, skipping")
		return
	}

	calendarID := s.deps.Cfg.GoogleCalendar.DefaultCalendarID
	var nNotified int
	for _, ua := range answers {
		if !ua.GCalEventID.Valid || ua.GCalEventID.String == "" {
			continue
		}

		// PATCH the calendar event with sendNotifications=true
		// This triggers a Google Calendar notification to the attendee
		if err := gcal.UpdateEventWithNotification(ctx, calendarID, ua.GCalEventID.String); err != nil {
			log.Warn("failed to trigger calendar notification",
				"answerID", ua.AnswerID, "gcalEventID", ua.GCalEventID.String, "error", err)
			continue
		}
		nNotified++
	}
	log.Info("completed", "notified", nNotified, "total", len(answers))
}
