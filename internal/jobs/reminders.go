package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/InCrowd/unified-qual-api/internal/integration"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
)

// remindIntervieweesDayBefore sends email reminders to confirmed interviewees
// whose interviews are scheduled for the next day.
// Matches Scala: CommsManager.RemindIntervieweesDayBefore
// Schedule: daily at 11am UTC
func (s *Scheduler) remindIntervieweesDayBefore() {
	ctx := context.Background()
	log := s.logger.With("job", "RemindIntervieweesDayBefore")
	log.Info("starting")

	if s.deps.JobsRepo == nil {
		log.Warn("skipped — no jobs repo configured")
		return
	}

	interviews, err := s.deps.JobsRepo.GetTomorrowInterviews(ctx)
	if err != nil {
		log.Error("failed to get tomorrow interviews", "error", err)
		return
	}
	log.Info("found interviews for tomorrow", "count", len(interviews))

	var nSent int
	for _, ir := range interviews {
		if err := s.sendInterviewReminder(ctx, log, ir, "day-before"); err != nil {
			log.Error("failed to send reminder",
				"timeSlotID", ir.TimeSlotID, "email", ir.UserEmail, "error", err)
			continue
		}
		nSent++
	}
	log.Info("completed", "sent", nSent, "total", len(interviews))
}

// remindInterviewees30MinBefore sends reminders 30 minutes before the interview.
// Matches Scala: CommsManager.RemindInterviewees30MinBefore
// Schedule: every minute (checks 28-32 min window)
func (s *Scheduler) remindInterviewees30MinBefore() {
	ctx := context.Background()
	log := s.logger.With("job", "RemindInterviewees30MinBefore")

	if s.deps.JobsRepo == nil {
		return
	}

	interviews, err := s.deps.JobsRepo.GetUpcomingInterviews(ctx)
	if err != nil {
		log.Error("failed to get upcoming interviews", "error", err)
		return
	}

	if len(interviews) == 0 {
		return
	}
	log.Info("found upcoming interviews", "count", len(interviews))

	var nSent int
	for _, ir := range interviews {
		if err := s.sendInterviewReminder(ctx, log, ir, "30-min-before"); err != nil {
			log.Error("failed to send reminder",
				"timeSlotID", ir.TimeSlotID, "email", ir.UserEmail, "error", err)
			continue
		}
		nSent++
	}
	log.Info("completed", "sent", nSent, "total", len(interviews))
}

func (s *Scheduler) sendInterviewReminder(ctx context.Context, log *slog.Logger, ir iris.InterviewReminder, reminderType string) error {
	nc := s.deps.Services.Notification
	if nc == nil || !nc.Configured() {
		return nil
	}

	joinLink := buildJoinLink(s.cfg.QualConfBaseURL, ir.ConferenceHash, ir.ParticipantHash)

	// Get telephone number from conference metadata (best-effort)
	phoneNumber := ""
	if ir.PhoneNumber.Valid {
		phoneNumber = ir.PhoneNumber.String
	}
	if phoneNumber == "" && s.deps.Services.Conference != nil {
		// TODO: fetch from ConferenceClient.GetMetadata if available
		_ = phoneNumber
	}

	subject := fmt.Sprintf("Reminder: Your interview for %s", ir.ProjectName)
	body := fmt.Sprintf(
		"This is a reminder for your upcoming interview for %s scheduled at %s. "+
			"Join using this link: %s",
		ir.ProjectName, ir.StartTime.Format("Monday, January 2 at 3:04 PM"), joinLink)

	if phoneNumber != "" {
		body += fmt.Sprintf(" Dial-in number: %s", phoneNumber)
	}

	if err := nc.SendEmail(ctx, integration.EmailMessage{
		To:          []string{ir.UserEmail},
		Subject:     subject,
		Body:        body,
		ContentType: "text/html",
	}); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	// TODO: SMS sending (commented out in Scala — implement when needed)

	return nil
}

func buildJoinLink(baseURL, conferenceHash, participantHash string) string {
	if baseURL == "" {
		return ""
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + conferenceHash
	}
	u.Path += conferenceHash
	q := u.Query()
	q.Set("commsHash", participantHash)
	u.RawQuery = q.Encode()
	return u.String()
}
