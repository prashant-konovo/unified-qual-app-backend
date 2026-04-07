package jobs

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// updateCalendarPushWatch renews the Google Calendar push watch channel to keep
// receiving calendar event notifications via webhook.
// Matches Scala: Scheduler.updateCalendarPushWatch
// Schedule: every 20 days
func (s *Scheduler) updateCalendarPushWatch() {
	ctx := context.Background()
	log := s.logger.With("job", "UpdateCalendarPushWatch")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	gcal := s.deps.Services.GoogleCal
	if gcal == nil || !gcal.Configured() {
		log.Warn("Google Calendar client not configured, skipping")
		return
	}

	existing, err := s.deps.JobsRepo.GetCalendarWatchConfig(ctx)
	if err != nil {
		log.Error("failed to get calendar watch config", "error", err)
		return
	}
	if existing == nil {
		log.Warn("no existing calendar watch found, skipping")
		return
	}

	// Generate new watch channel
	newWatchID := uuid.New().String()
	expiration := time.Now().Add(21 * 24 * time.Hour)

	// Register new watch
	webhookURL := s.deps.Cfg.AWS.ICApiURL + "/v1/googlecalendar/notifications"
	newResourceID, err := gcal.RegisterWatch(ctx, existing.CalendarID, newWatchID, webhookURL, expiration)
	if err != nil {
		log.Error("failed to register new calendar watch",
			"calendarID", existing.CalendarID, "error", err)
		return
	}

	// Deregister old watch (best-effort)
	if existing.WatchID != "" {
		if err := gcal.DeregisterWatch(ctx, existing.WatchID, existing.ResourceID); err != nil {
			log.Warn("failed to deregister old calendar watch",
				"watchID", existing.WatchID, "error", err)
		}
	}

	// Persist new watch in DB
	if err := s.deps.JobsRepo.UpdateCalendarWatch(ctx, newWatchID, newResourceID, expiration); err != nil {
		log.Error("failed to update calendar watch in DB", "error", err)
		return
	}

	log.Info("calendar push watch renewed",
		"calendarID", existing.CalendarID,
		"newWatchID", newWatchID,
		"expiration", expiration.Format(time.RFC3339))
}
